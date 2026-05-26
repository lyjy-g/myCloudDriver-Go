#!/usr/bin/env python3
import argparse
import concurrent.futures
import hashlib
import json
import os
import re
import signal
import subprocess
import threading
import time
from pathlib import Path

import requests


DEFAULT_BASE = "http://127.0.0.1:8080"
DEFAULT_USER = "admin"
DEFAULT_PASS = "admin"
CHUNK_SIZE = 5 * 1024 * 1024


def api_get(base, path, token=None, workspace_id=None, storage_id=None):
    headers = {"Accept": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if workspace_id:
        headers["X-Workspace-Id"] = workspace_id
    if storage_id:
        headers["X-Storage-Setting-Id"] = storage_id
    resp = requests.get(f"{base}{path}", headers=headers, timeout=120)
    resp.raise_for_status()
    return resp.json()


def api_post(base, path, token=None, workspace_id=None, storage_id=None, json_body=None, files=None):
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if workspace_id:
        headers["X-Workspace-Id"] = workspace_id
    if storage_id:
        headers["X-Storage-Setting-Id"] = storage_id
    if files is None:
        headers["Content-Type"] = "application/json"
    resp = requests.post(
        f"{base}{path}",
        headers=headers,
        json=json_body if files is None else None,
        files=files,
        timeout=120,
    )
    resp.raise_for_status()
    return resp.json()


def login(base, user, password):
    resp = api_post(base, "/apis/auth/login", json_body={"username": user, "password": password})
    data = resp.get("data", resp)
    return data["token"], data["workspaceId"]


def list_settings(base, token, workspace_id):
    # 当前后端的 storage 设置接口需要 workspace 上下文，因此这里强制带上工作空间头。
    resp = api_get(base, "/apis/storage/platform/settings", token=token, workspace_id=workspace_id)
    return resp.get("data", resp)


def pick_setting(settings):
    for item in settings:
        if item.get("enabled") or item.get("active"):
            return item.get("ID") or item.get("id") or item.get("settingId") or ""
    if not settings:
        return ""
    first = settings[0]
    return first.get("ID") or first.get("id") or first.get("settingId") or ""


def set_active_setting(base, token, workspace_id, setting_id):
    api_post(base, f"/apis/storage/settings/{setting_id}/select", token=token, workspace_id=workspace_id, json_body={})


def sha256_hex(data):
    return hashlib.sha256(data).hexdigest()


def build_file_bytes(mode, task_idx, file_size):
    seed = f"{mode}:{task_idx}".encode()
    block = hashlib.sha256(seed).digest()
    repeat = file_size // len(block) + 1
    return (block * repeat)[:file_size]


def find_api_pid():
    # 取 go run 对应的实际进程，便于采样 CPU/内存。
    try:
        out = subprocess.check_output(
            ["bash", "-lc", "pgrep -af 'cmd/api' | head -n 1 | awk '{print $1}'"],
            text=True,
        ).strip()
        return int(out) if out else None
    except Exception:
        return None


def mysql_questions():
    try:
        out = subprocess.check_output(
            ["bash", "-lc", "mysql -N -uroot -pmyclouddrive -h127.0.0.1 -e \"SHOW GLOBAL STATUS LIKE 'Questions';\""],
            text=True,
            stderr=subprocess.DEVNULL,
        )
        m = re.search(r"Questions\s+(\d+)", out)
        return int(m.group(1)) if m else 0
    except Exception:
        return 0


def redis_commands():
    try:
        out = subprocess.check_output(
            ["bash", "-lc", "redis-cli -h 127.0.0.1 INFO stats"],
            text=True,
            stderr=subprocess.DEVNULL,
        )
        m = re.search(r"total_commands_processed:(\d+)", out)
        return int(m.group(1)) if m else 0
    except Exception:
        return 0


def sampler(pid, stop, holder):
    peak_cpu = 0.0
    peak_rss = 0
    while not stop.is_set():
        try:
            out = subprocess.check_output(
                ["bash", "-lc", f"ps -o %cpu=,rss= -p {pid}"],
                text=True,
                stderr=subprocess.DEVNULL,
            ).strip()
            if out:
                cpu_s, rss_s = out.split()
                peak_cpu = max(peak_cpu, float(cpu_s))
                peak_rss = max(peak_rss, int(rss_s) * 1024)
        except Exception:
            pass
        time.sleep(1)
    holder["cpu"] = peak_cpu
    holder["rss"] = peak_rss


def upload_task(base, token, workspace_id, storage_id, mode, task_idx, file_size, total_parts):
    file_name = f"{mode}-task-{task_idx}.bin"
    file_bytes = build_file_bytes(mode, task_idx, file_size)
    file_hash = sha256_hex(file_bytes)
    precheck = api_post(
        base,
        "/apis/transfer/check",
        token=token,
        workspace_id=workspace_id,
        storage_id=storage_id,
        json_body={
            "fileName": file_name,
            "fileSize": file_size,
            "fileHash": file_hash,
            "totalParts": total_parts,
            "contentType": "application/octet-stream",
            "parentId": "root",
        },
    )
    data = precheck.get("data", precheck)
    if data.get("skipUpload"):
        return {"skipped": True, "taskId": data.get("taskId", "")}

    task_id = data.get("taskId") or data.get("uploadId")
    if not task_id:
        raise RuntimeError("missing taskId")

    if mode == "sse":
        # 这里只验证 SSE 端点能返回任务快照，不做完整实时推送断言。
        sse_resp = requests.get(
            f"{base}/apis/transfer/stream/{task_id}",
            headers={
                "Authorization": f"Bearer {token}",
                "X-Workspace-Id": workspace_id,
                "X-Storage-Setting-Id": storage_id,
                "Accept": "text/event-stream",
            },
            timeout=120,
            stream=True,
        )
        sse_resp.raise_for_status()
        sse_resp.close()

    for part_idx in range(1, total_parts + 1):
        start = (part_idx - 1) * CHUNK_SIZE
        end = min(file_size, part_idx * CHUNK_SIZE)
        chunk = file_bytes[start:end]
        chunk_hash = sha256_hex(chunk)
        files = {
            "file": (f"part-{part_idx}.bin", chunk, "application/octet-stream"),
        }
        chunk_resp = api_post(
            base,
            f"/apis/transfer/chunk?taskId={task_id}&chunkIndex={part_idx}&chunkSha256={chunk_hash}",
            token=token,
            workspace_id=workspace_id,
            storage_id=storage_id,
            files=files,
        )
        if mode in ("poll", "sse"):
            api_get(base, f"/apis/transfer/task/{task_id}", token=token, workspace_id=workspace_id, storage_id=storage_id)
        elif mode in ("chunk-progress", "precheck-progress"):
            # 返回值里已经包含进度，这里保持只读，不额外轮询。
            _ = chunk_resp

    merged = api_post(
        base,
        f"/apis/transfer/merge/{task_id}",
        token=token,
        workspace_id=workspace_id,
        storage_id=storage_id,
        json_body={"taskId": task_id, "uploadId": task_id},
    )
    return {"taskId": task_id, "merged": bool(merged)}


def run_mode(base, token, workspace_id, storage_id, mode, file_size, task_count):
    total_parts = (file_size + CHUNK_SIZE - 1) // CHUNK_SIZE
    mysql_start = mysql_questions()
    redis_start = redis_commands()
    pid = find_api_pid()
    metrics = {"cpu": 0.0, "rss": 0}
    stop = threading.Event()
    th = None
    if pid:
        th = threading.Thread(target=sampler, args=(pid, stop, metrics), daemon=True)
        th.start()
    start = time.time()
    errors = 0
    details = []
    # 这里改成真正并发执行多个上传任务，才能更接近设计稿里的并发任务压测场景。
    with concurrent.futures.ThreadPoolExecutor(max_workers=task_count) as executor:
        futures = [
            executor.submit(upload_task, base, token, workspace_id, storage_id, mode, i, file_size, total_parts)
            for i in range(task_count)
        ]
        for future in concurrent.futures.as_completed(futures):
            try:
                details.append(future.result())
            except Exception as exc:
                errors += 1
                details.append({"error": str(exc)})
    elapsed = time.time() - start
    stop.set()
    if th:
        th.join(timeout=5)
    mysql_end = mysql_questions()
    redis_end = redis_commands()
    total_requests = task_count * (total_parts + 3)  # precheck + chunks + merge + 轮询/SSE 的近似值
    if mode == "poll":
        total_requests += task_count * total_parts
    elif mode == "sse":
        total_requests += task_count * 2
    elif mode == "chunk-progress":
        total_requests += task_count
    elif mode == "precheck-progress":
        total_requests += task_count
    return {
        "mode": mode,
        "elapsed_sec": round(elapsed, 2),
        "total_requests_est": total_requests,
        "server_cpu_peak_pct": round(metrics["cpu"], 2),
        "server_mem_peak_bytes": metrics["rss"],
        "mysql_questions_delta": max(0, mysql_end - mysql_start),
        "redis_commands_delta": max(0, redis_end - redis_start),
        "errors": errors,
        "error_rate": round(errors / task_count, 4),
        "sample": details[:3],
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default=DEFAULT_BASE)
    parser.add_argument("--user", default=DEFAULT_USER)
    parser.add_argument("--password", default=DEFAULT_PASS)
    parser.add_argument("--file-size-mib", type=int, default=100)
    parser.add_argument("--tasks", type=int, default=10)
    parser.add_argument("--out", default="")
    args = parser.parse_args()

    base = args.base.rstrip("/")
    file_size = args.file_size_mib * 1024 * 1024
    token, workspace_id = login(base, args.user, args.password)
    settings = list_settings(base, token, workspace_id)
    setting_id = pick_setting(settings)
    if not setting_id:
      raise RuntimeError("no storage setting found")
    set_active_setting(base, token, workspace_id, setting_id)

    rows = []
    rows.append("# 上传进度压测记录\n")
    rows.append(f"- 时间: {time.strftime('%Y-%m-%d %H:%M:%S')}\n")
    rows.append(f"- 基础地址: {base}\n")
    rows.append(f"- 工作空间: {workspace_id}\n")
    rows.append(f"- 存储配置: {setting_id}\n")
    rows.append("- 计划规模: 10G / 5MB / 并发10\n")
    rows.append(f"- 实测规模: {args.file_size_mib}MiB / 5MB / 并发{args.tasks}\n")
    rows.append("- 说明: 本机执行采用 100MiB 文件做全链路验证，避免 10G 在当前环境下耗时过长。\n\n")
    rows.append("## 指标\n")
    for mode in ["poll", "chunk-progress", "precheck-progress", "sse"]:
        result = run_mode(base, token, workspace_id, setting_id, mode, file_size, args.tasks)
        rows.append(f"\n### {mode}\n")
        for key in ["elapsed_sec", "total_requests_est", "server_cpu_peak_pct", "server_mem_peak_bytes", "mysql_questions_delta", "redis_commands_delta", "errors", "error_rate"]:
            rows.append(f"- {key}: {result[key]}\n")
        rows.append(f"- sample: {json.dumps(result['sample'], ensure_ascii=False)}\n")

    out = Path(args.out) if args.out else Path(__file__).resolve().parents[1] / "test_files" / f"upload-benchmark-{time.strftime('%Y%m%d-%H%M%S')}.md"
    out.write_text("".join(rows), encoding="utf-8")
    print(out)


if __name__ == "__main__":
    main()
