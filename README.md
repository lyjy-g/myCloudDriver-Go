# myclouddrive-go

Go 版本 MyCloudDrive 后端骨架。

## Storage 接口迁移（OpenAPI 3 + Spec First）

已落地 storage 域的契约优先方案：

1. 契约文件：`api/openapi/storage.openapi.yaml`
2. 生成配置：`api/openapi/oapi-codegen.storage.yaml`
3. 生成代码：`internal/storage/api/storage.gen.go`
4. handler 实现：`internal/storage/api/handler.go`
5. 业务占位：`internal/storage/service/placeholder_service.go`
6. CI 校验：`.github/workflows/openapi-sync.yml`

常用命令：

```bash
make openapi-generate
make openapi-check
make run-gin
go test ./...
```

说明：

- 接口变更先改 OpenAPI，再生成代码，再实现业务。
- 生成代码应提交到仓库，方便前后端和测试基于同一契约协作。
- `gin + gorm + go-redis` 工程化示例说明见 `docs/gin-gorm-redis-example.md`。

## 模块化扩展

`cmd/api` 现已采用模块注册启动：`app.NewServer(...modules)`。

当前已注册模块：

1. `storage`（示例完整模块）
2. `user`（占位）
3. `file`（占位）
4. `share`（占位）

后续新增服务时，按 `internal/<service>/{module,api,service,repository,model}` 结构创建并在 `cmd/api/main.go` 注册即可。

## 生成代码约定

按目录自治生成逻辑：

1. OpenAPI 生成：`internal/<service>/api/generate.go`
2. GORM model 生成：`internal/<service>/model/generate.go`
3. 每个目录只维护自己的生成命令，不跨目录写生成逻辑

当前 `storage` 已按该约定落地：

1. `internal/storage/api/generate.go`
2. `internal/storage/api/storage.gen.go`
3. `internal/storage/model/generate.go`

## 目录说明

- `cmd/api`: API 进程入口。
- `internal/app`: 应用装配与依赖注入。
- `internal/framework`: 基础设施与横切能力。
- `internal/domain`: 领域模型。
- `internal/service`: 业务服务层。
- `internal/repository`: 数据访问抽象。
- `internal/transport/http`: HTTP 路由与处理器。
- `pkg/types`: 可复用稳定类型。
- `configs`: 配置文件样例。
- `migrations`: 数据库迁移脚本。
