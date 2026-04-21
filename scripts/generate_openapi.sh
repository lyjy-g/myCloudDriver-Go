#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

OAPI_CODEGEN_BIN="${OAPI_CODEGEN_BIN:-$(command -v oapi-codegen || true)}"
if [[ -z "$OAPI_CODEGEN_BIN" ]]; then
  echo "oapi-codegen 未安装，请先执行: go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest" >&2
  exit 1
fi

# 基于 OpenAPI 契约生成 storage 接口模型、server interface、client。
"$OAPI_CODEGEN_BIN" -config api/openapi/oapi-codegen.storage.yaml api/openapi/storage.openapi.yaml

echo "OpenAPI 代码生成完成: internal/storage/api/storage.gen.go"
