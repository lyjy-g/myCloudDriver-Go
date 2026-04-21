#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

./scripts/generate_openapi.sh

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "当前目录不是 git 仓库，已完成本地生成；无法执行 diff 同步校验。" >&2
  exit 0
fi

if ! git diff --quiet -- \
  internal/storage/api/gen/storage.gen.go \
  internal/user/api/gen/user.gen.go \
  internal/file/api/gen/file.gen.go \
  internal/share/api/gen/share.gen.go; then
  echo "检测到 OpenAPI 生成代码未同步，请提交最新生成结果。" >&2
  git --no-pager diff -- \
    internal/storage/api/gen/storage.gen.go \
    internal/user/api/gen/user.gen.go \
    internal/file/api/gen/file.gen.go \
    internal/share/api/gen/share.gen.go
  exit 1
fi

echo "OpenAPI 契约与生成代码已同步。"
