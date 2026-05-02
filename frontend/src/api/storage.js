/**
 * 前端 API 适配层。
 * 目标：无后端改动前提下，适配当前 Go 路由与响应结构。
 */

export const AUTH_TOKEN_KEY = "mcd-console-auth-token";
let currentWorkspaceId = "";
let currentStorageSettingId = "";

export function setCurrentWorkspaceId(workspaceId) {
  currentWorkspaceId = String(workspaceId || "").trim();
}

export function setCurrentStorageSettingId(settingId) {
  currentStorageSettingId = String(settingId || "").trim();
}

export function saveAuthToken(token) {
  if (!token) {
    return;
  }
  window.localStorage.setItem(AUTH_TOKEN_KEY, token);
}

export function getAuthToken() {
  return window.localStorage.getItem(AUTH_TOKEN_KEY) || "";
}

export function clearAuthToken() {
  window.localStorage.removeItem(AUTH_TOKEN_KEY);
}

function createIdempotencyKey() {
  if (typeof crypto !== "undefined" && crypto?.randomUUID) {
    return crypto.randomUUID();
  }
  return `idem_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function buildHeaders(headers = {}, withAuth = true, withIdempotency = false) {
  const finalHeaders = { ...headers };
  if (withAuth) {
    const token = getAuthToken();
    if (token) {
      finalHeaders.Authorization = `Bearer ${token}`;
    }
  }
  if (withIdempotency) {
    finalHeaders["Idempotency-Key"] = createIdempotencyKey();
  }
  if (currentWorkspaceId) {
    finalHeaders["X-Workspace-Id"] = currentWorkspaceId;
  }
  if (currentStorageSettingId) {
    finalHeaders["X-Storage-Setting-Id"] = currentStorageSettingId;
  }
  return finalHeaders;
}

function unwrapPayload(payload) {
  if (payload == null || typeof payload !== "object") {
    return payload;
  }
  if (Object.prototype.hasOwnProperty.call(payload, "data")) {
    return payload.data;
  }
  return payload;
}

function normalizeIdentifier(identifier = "") {
  return String(identifier || "").trim().toLowerCase();
}

function parseSettingConfig(item) {
  const rawConfig = item?.configJson || item?.configData || "{}";
  if (typeof rawConfig === "string") {
    try {
      return JSON.parse(rawConfig || "{}");
    } catch {
      return {};
    }
  }
  if (rawConfig && typeof rawConfig === "object") {
    return rawConfig;
  }
  return {};
}

function readErrorMessage(payload, fallback = "请求失败") {
  if (!payload || typeof payload !== "object") {
    return fallback;
  }
  return payload.msg || payload.message || payload.error || fallback;
}

function isSuccessCode(code) {
  return code === undefined || code === 200 || code === "200" || code === "OK";
}

async function requestJson(method, urls, {
  body,
  withAuth = true,
  withIdempotency = false
} = {}) {
  const candidates = Array.isArray(urls) ? urls : [urls];
  let lastError = null;

  for (const url of candidates) {
    const response = await fetch(url, {
      method,
      headers: buildHeaders(
        body === undefined
          ? { Accept: "application/json" }
          : { Accept: "application/json", "Content-Type": "application/json" },
        withAuth,
        withIdempotency
      ),
      body: body === undefined ? undefined : JSON.stringify(body)
    });

    // 404/405 尝试下一个兼容路径。
    if (response.status === 404 || response.status === 405) {
      lastError = new Error(`endpoint not found: ${url}`);
      continue;
    }

    const text = await response.text();
    let payload = null;
    if (text) {
      try {
        payload = JSON.parse(text);
      } catch {
        payload = null;
      }
    }

    if (!response.ok) {
      throw new Error(readErrorMessage(payload, `请求失败(${response.status})`));
    }

    if (payload && Object.prototype.hasOwnProperty.call(payload, "code") && !isSuccessCode(payload.code)) {
      throw new Error(readErrorMessage(payload));
    }

    if (payload && payload.success === false) {
      throw new Error(readErrorMessage(payload));
    }

    return payload;
  }

  throw lastError || new Error("请求失败");
}

export function getJson(url, withAuth = true) {
  return requestJson("GET", url, { withAuth });
}

export function postJson(url, payload, withAuth = true) {
  return requestJson("POST", url, { body: payload, withAuth });
}

export function putJson(url, payload, withAuth = true) {
  return requestJson("PUT", url, { body: payload, withAuth });
}

export function fetchPlatforms(baseUrl) {
  return requestJson("GET", `${baseUrl}/apis/storage/platforms`);
}

export function queryAgent(baseUrl, payload) {
  return requestJson("POST", `${baseUrl}/apis/agent/query`, {
    body: {
      query: payload?.query || "",
      scope: payload?.scope || "auto",
      mode: payload?.mode || "search",
      workspaceId: payload?.workspaceId || "",
      storageSettingId: payload?.storageSettingId || "",
      traceId: payload?.traceId || ""
    }
  });
}

export function confirmAgentAction(baseUrl, traceId, payload = {}) {
  return requestJson("POST", `${baseUrl}/apis/agent/confirm/${encodeURIComponent(traceId)}`, {
    body: {
      confirmed: payload.confirmed !== false,
      planId: payload.planId || ""
    }
  });
}

/**
 * 流式 Agent 查询（SSE）。
 * @param {string} baseUrl
 * @param {object} payload 同 queryAgent
 * @param {function} onEvent 回调 (eventName, data) => void
 * @returns {AbortController} 用于取消请求
 */
export function streamAgentQuery(baseUrl, payload, onEvent) {
  const controller = new AbortController();
  const body = JSON.stringify({
    query: payload?.query || "",
    scope: payload?.scope || "auto",
    mode: payload?.mode || "search",
    workspaceId: payload?.workspaceId || "",
    storageSettingId: payload?.storageSettingId || "",
    traceId: payload?.traceId || ""
  });

  const extraHeaders = {};
  if (currentWorkspaceId) { extraHeaders["X-Workspace-Id"] = currentWorkspaceId; }
  if (currentStorageSettingId) { extraHeaders["X-Storage-Setting-Id"] = currentStorageSettingId; }

  fetch(`${baseUrl}/apis/agent/stream`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Accept": "text/event-stream",
      "Authorization": getAuthToken() ? `Bearer ${getAuthToken()}` : "",
      ...extraHeaders
    },
    body,
    signal: controller.signal
  }).then(async (response) => {
    if (!response.ok) {
      const text = await response.text();
      onEvent("error", { message: text || `HTTP ${response.status}` });
      return;
    }
    const reader = response.body?.getReader();
    if (!reader) {
      onEvent("error", { message: "no response body" });
      return;
    }
    const decoder = new TextDecoder();
    let buffer = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      // 按行解析 SSE
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";
      let currentEvent = "";
      for (const line of lines) {
        if (line.startsWith("event: ")) {
          currentEvent = line.slice(7).trim();
        } else if (line.startsWith("data: ")) {
          const raw = line.slice(6).trim();
          try {
            const data = JSON.parse(raw);
            onEvent(currentEvent || "message", data);
          } catch {
            onEvent(currentEvent || "message", { raw });
          }
          currentEvent = "";
        }
      }
    }
  }).catch((err) => {
    if (err.name !== "AbortError") {
      onEvent("error", { message: err.message });
    }
  });

  return controller;
}

export async function fetchStorageSettings(baseUrl) {
  const listResp = await requestJson("GET", `${baseUrl}/apis/storage/platform/settings`);
  const settings = unwrapPayload(listResp) || [];
  if (!Array.isArray(settings)) {
    return { code: 200, msg: "success", data: [] };
  }
  return {
    code: 200,
    msg: "success",
    data: settings.map((item) => {
      const config = parseSettingConfig(item);
      const identifier = item.identifier || item.platformIdentifier || "Local";
      const normalized = normalizeIdentifier(identifier);
      return {
        settingId: item.id || item.settingId || "",
        storageSettingName: item.storageSettingName || item.name || "",
        identifier,
        name: item.storageSettingName || item.name || identifier || "Local",
        active: item.active === true || item.active === 1 || item.enabled === true || item.enabled === 1,
        basePath: config.basePath || config.rootPath || "",
        namespace: config.namespace || "",
        baseUrl: config.baseUrl || config.publicBaseUrl || "",
        endpoint: config.endpoint || "",
        region: config.region || "",
        bucket: config.bucket || "",
        accessKeyId: config.access_key_id || config.accessKeyId || "",
        secretAccessKey: config.secret_access_key || config.secretAccessKey || "",
        prefix: config.prefix || "",
        useSSL: config.use_ssl === true || config.useSSL === true,
        pathStyle: normalized === "s3" ? config.path_style !== false : (config.path_style === true || config.pathStyle === true),
        config,
        raw: item
      };
    })
  };
}

export async function fetchActiveStorage(baseUrl) {
  const listResp = await fetchStorageSettings(baseUrl);
  const settings = unwrapPayload(listResp) || [];
  const active = settings.find((item) => item.active) || settings[0] || null;
  return {
    code: 200,
    msg: "success",
    data: active
  };
}

export async function updateActiveStorage(baseUrl, payload) {
  const identifier = payload.identifier || "Local";
  const normalized = normalizeIdentifier(identifier);
  let config = {};
  if (normalized === "s3") {
    config = {
      endpoint: payload.endpoint || "",
      region: payload.region || "us-east-1",
      bucket: payload.bucket || "",
      access_key_id: payload.accessKeyId || "",
      secret_access_key: payload.secretAccessKey || "",
      prefix: payload.prefix || "",
      use_ssl: Boolean(payload.useSSL),
      path_style: payload.pathStyle !== false
    };
  } else {
    config = {
      namespace: payload.namespace || "",
      baseUrl: payload.baseUrl || ""
    };
  }
  const configJson = JSON.stringify(config);

  let settingId = payload.settingId;
  const storageSettingName = String(payload.storageSettingName || "").trim();
  if (!settingId || settingId === "local-default") {
    const createResp = await requestJson("POST", `${baseUrl}/apis/storage/settings`, {
      body: { storageSettingName, identifier, configJson }
    });
    const created = unwrapPayload(createResp) || {};
    settingId = created.id || created.settingId;
  }

  // 可编辑时先更新配置。
  if (settingId && payload.settingId && payload.settingId !== "local-default") {
    try {
      await requestJson("PUT", `${baseUrl}/apis/storage/settings/${encodeURIComponent(settingId)}`, {
        body: { storageSettingName, configJson }
      });
    } catch {
      // 部分后端版本没有更新路由，忽略并继续启用流程。
    }
  }

  try {
    await requestJson("POST", `${baseUrl}/apis/storage/settings/${encodeURIComponent(settingId)}/1`, {});
  } catch {
    const createResp = await requestJson("POST", `${baseUrl}/apis/storage/settings`, {
      body: { storageSettingName, identifier, configJson }
    });
    const created = unwrapPayload(createResp) || {};
    settingId = created.id || created.settingId || settingId;
    await requestJson("POST", `${baseUrl}/apis/storage/settings/${encodeURIComponent(settingId)}/1`, {});
  }

  return {
    code: 200,
    msg: "success",
    data: {
      settingId,
      storageSettingName,
      identifier,
      basePath: payload.basePath || "",
      baseUrl: payload.baseUrl || ""
    }
  };
}

export function activateStorageSetting(baseUrl, settingId) {
  return requestJson("POST", `${baseUrl}/apis/storage/settings/${encodeURIComponent(settingId)}/1`, {});
}

export function login(baseUrl, payload) {
  return requestJson("POST", `${baseUrl}/apis/auth/login`, { body: payload, withAuth: false });
}

export function logout(baseUrl) {
  return requestJson("POST", `${baseUrl}/apis/auth/logout`, { body: {} });
}

export function fetchCurrentUser(baseUrl) {
  return requestJson("GET", [
    `${baseUrl}/apis/user/info`,
    `${baseUrl}/apis/auth/me`
  ]);
}

export function fetchWorkspaces(baseUrl) {
  return requestJson("GET", [
    `${baseUrl}/apis/user/workspaces`,
    `${baseUrl}/apis/workspaces`
  ]);
}

export async function fetchActiveWorkspace(baseUrl) {
  const listResp = await fetchWorkspaces(baseUrl);
  const workspaces = unwrapPayload(listResp) || [];
  if (!Array.isArray(workspaces) || workspaces.length === 0) {
    return { code: "OK", message: "success", data: null };
  }
  const active = workspaces.find((item) => item?.isDefault) || workspaces[0];
  return { code: "OK", message: "success", data: active };
}

export function updateActiveWorkspace(baseUrl, payload) {
  const workspaceId = encodeURIComponent(payload?.workspaceId || "");
  return requestJson("PUT", [
    `${baseUrl}/apis/user/default-workspace/${workspaceId}`,
    `${baseUrl}/apis/workspaces/active`
  ], { body: payload });
}

export function fetchFileList(baseUrl) {
  return requestJson("GET", `${baseUrl}/apis/file/list`);
}

export function fetchEntriesByParent(baseUrl, parentId) {
  const encodedParentId = encodeURIComponent(parentId || "");
  return requestJson("GET", `${baseUrl}/apis/file/list?parentId=${encodedParentId}`);
}

export function createDirectory(baseUrl, payload) {
  return requestJson("POST", `${baseUrl}/apis/file/directory`, {
    body: payload,
    withIdempotency: true
  });
}

export function fetchDirectories(baseUrl, parentId) {
  const encodedParentId = encodeURIComponent(parentId || "");
  return requestJson("GET", `${baseUrl}/apis/file/dirs?parentId=${encodedParentId}`);
}

export function renameDirectory(baseUrl, directoryId, name) {
  return requestJson("PUT", `${baseUrl}/apis/file/${encodeURIComponent(directoryId)}/rename`, {
    body: { newName: name },
    withIdempotency: true
  });
}

export function deleteDirectory(baseUrl, directoryId) {
  return requestJson("DELETE", `${baseUrl}/apis/file`, {
    body: [directoryId],
    withIdempotency: true
  });
}

export function renameFile(baseUrl, fileId, name) {
  return requestJson("PUT", `${baseUrl}/apis/file/${encodeURIComponent(fileId)}/rename`, {
    body: { newName: name },
    withIdempotency: true
  });
}

export function moveFile(baseUrl, fileId, targetParentId) {
  return requestJson("PUT", `${baseUrl}/apis/file/moves`, {
    body: { fileIds: [fileId], targetParentId },
    withIdempotency: true
  });
}

export function deleteFile(baseUrl, fileId) {
  return requestJson("DELETE", `${baseUrl}/apis/file`, {
    body: [fileId],
    withIdempotency: true
  });
}

export function fetchRecycleBin(baseUrl) {
  return requestJson("GET", `${baseUrl}/apis/file/recycle/pages?page=1&size=200`);
}

export function restoreRecord(baseUrl, fileId) {
  return requestJson("PUT", `${baseUrl}/apis/file/recycles`, {
    body: [fileId],
    withIdempotency: true
  });
}

export function precheckUpload(baseUrl, payload) {
  return requestJson("POST", [
    `${baseUrl}/apis/transfer/check`,
    `${baseUrl}/apis/upload/precheck`
  ], { body: payload });
}

export async function uploadPart(baseUrl, payload) {
  const taskId = payload.taskId || payload.uploadId;
  const chunkIndex = payload.chunkIndex ?? payload.partNumber;
  const chunkMd5 = payload.chunkMd5 || payload.fileHash || "";

  const formData = new FormData();
  formData.append("taskId", taskId);
  formData.append("uploadId", taskId);
  formData.append("chunkIndex", String(chunkIndex));
  formData.append("partNumber", String(chunkIndex));
  formData.append("totalParts", String(payload.totalParts));
  formData.append("fileHash", payload.fileHash);
  formData.append("file", payload.file);

  const candidates = [
    `${baseUrl}/apis/transfer/chunk?taskId=${encodeURIComponent(taskId)}&chunkIndex=${encodeURIComponent(String(chunkIndex))}&chunkMd5=${encodeURIComponent(chunkMd5)}`,
    `${baseUrl}/apis/upload/part`
  ];

  let lastError = null;
  for (const url of candidates) {
    const response = await fetch(url, {
      method: "POST",
      headers: buildHeaders({}, true, false),
      body: formData
    });

    if (response.status === 404 || response.status === 405) {
      lastError = new Error(`endpoint not found: ${url}`);
      continue;
    }

    const text = await response.text();
    let payloadData = null;
    if (text) {
      try {
        payloadData = JSON.parse(text);
      } catch {
        payloadData = null;
      }
    }

    if (!response.ok) {
      throw new Error(readErrorMessage(payloadData, `分片上传失败(${response.status})`));
    }
    if (payloadData?.success === false) {
      throw new Error(readErrorMessage(payloadData, "分片上传失败"));
    }
    return payloadData;
  }

  throw lastError || new Error("分片上传失败");
}

export function mergeUpload(baseUrl, payload) {
  const taskId = payload.taskId || payload.uploadId;
  return requestJson("POST", [
    `${baseUrl}/apis/transfer/merge/${encodeURIComponent(taskId)}`,
    `${baseUrl}/apis/upload/merge`
  ], { body: payload });
}

export async function downloadFile(baseUrl, fileId) {
  const response = await fetch(`${baseUrl}/apis/transfer/download/${encodeURIComponent(fileId)}`, {
    method: "GET",
    headers: buildHeaders({}, true, false)
  });
  if (!response.ok) {
    const text = await response.text();
    let payload = null;
    try {
      payload = text ? JSON.parse(text) : null;
    } catch {
      payload = null;
    }
    throw new Error(readErrorMessage(payload, `下载失败(${response.status})`));
  }
  const blob = await response.blob();
  let fileName = "download.bin";
  const disposition = response.headers.get("Content-Disposition") || response.headers.get("content-disposition") || "";
  // 优先支持 RFC 5987: filename*=UTF-8''xxx
  const m5987 = disposition.match(/filename\*\s*=\s*([^;]+)/i);
  if (m5987 && m5987[1]) {
    const raw = m5987[1].trim().replace(/^UTF-8''/i, "").replace(/^\"|\"$/g, "");
    try {
      fileName = decodeURIComponent(raw);
    } catch {
      fileName = raw;
    }
  } else {
    const m = disposition.match(/filename\s*=\s*\"?([^\";]+)\"?/i);
    if (m && m[1]) {
      fileName = m[1];
    }
  }
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  setTimeout(() => URL.revokeObjectURL(url), 2000);
}

export function rebuildFileIndexes(baseUrl) {
  return requestJson("POST", `${baseUrl}/apis/files/maintenance/rebuild-indexes`, { body: {} });
}

export function createShare(baseUrl, payload) {
  return requestJson("POST", `${baseUrl}/apis/share/create`, { body: payload });
}

export function fetchMyShares(baseUrl) {
  return requestJson("GET", `${baseUrl}/apis/share/pages`);
}

export function accessPublicShare(baseUrl, shareId, shareCode) {
  return requestJson("POST", `${baseUrl}/apis/share/verify/code`, {
    body: { shareId, shareCode },
    withAuth: false
  }).then(() =>
    requestJson("POST", `${baseUrl}/apis/shares/public/${encodeURIComponent(shareId)}/access`, {
      body: { shareCode },
      withAuth: false
    })
  );
}

export async function downloadPublicShareFile(baseUrl, shareId, fileId, shareCode) {
  const qs = new URLSearchParams();
  const normalizedShareCode = String(shareCode || "").trim();
  if (normalizedShareCode) {
    qs.set("shareCode", normalizedShareCode);
  }
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  const response = await fetch(`${baseUrl}/apis/share/${encodeURIComponent(shareId)}/download/${encodeURIComponent(fileId)}${suffix}`, {
    method: "GET"
  });
  if (!response.ok) {
    const text = await response.text();
    let payload = null;
    try {
      payload = text ? JSON.parse(text) : null;
    } catch {
      payload = null;
    }
    throw new Error(readErrorMessage(payload, `下载失败(${response.status})`));
  }
  const blob = await response.blob();
  let fileName = "download.bin";
  const disposition = response.headers.get("Content-Disposition") || response.headers.get("content-disposition") || "";
  const m5987 = disposition.match(/filename\*\s*=\s*([^;]+)/i);
  if (m5987 && m5987[1]) {
    const raw = m5987[1].trim().replace(/^UTF-8''/i, "").replace(/^\"|\"$/g, "");
    try {
      fileName = decodeURIComponent(raw);
    } catch {
      fileName = raw;
    }
  } else {
    const m = disposition.match(/filename\s*=\s*\"?([^\";]+)\"?/i);
    if (m && m[1]) {
      fileName = m[1];
    }
  }
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  setTimeout(() => URL.revokeObjectURL(url), 2000);
}

export function fetchShareDetail(baseUrl, shareId) {
  return requestJson("GET", `${baseUrl}/apis/share/${encodeURIComponent(shareId)}`);
}

export function updateShare(baseUrl, shareId, payload) {
  return requestJson("PUT", `${baseUrl}/apis/share/${encodeURIComponent(shareId)}`, { body: payload });
}

/**
 * 获取 Agent 对话历史。
 * @param {string} baseUrl
 * @param {object} opts - { before?: string, size?: number }
 * @returns {Promise<{items: Array, hasMore: boolean}>}
 */
/**
 * 停止正在执行的流式 Agent 查询。
 * @param {string} baseUrl
 * @param {string} traceId
 * @returns {Promise<object>}
 */
export function stopAgentQuery(baseUrl, traceId) {
  return requestJson("POST", `${baseUrl}/apis/agent/stop/${encodeURIComponent(traceId)}`);
}

export function fetchAgentHistory(baseUrl, opts = {}) {
  const params = new URLSearchParams();
  if (opts.before) params.set("before", opts.before);
  if (opts.size) params.set("size", String(opts.size));
  const qs = params.toString();
  const url = `${baseUrl}/apis/agent/session/history${qs ? "?" + qs : ""}`;
  return requestJson("GET", url).then((res) => res?.data || res || { items: [], hasMore: false });
}
