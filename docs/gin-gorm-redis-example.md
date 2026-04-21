# Gin + GORM + go-redis 工程化示例说明

这个示例用于演示：在 **OpenAPI 契约优先**前提下，如何组合 `gin`、`gorm`、`go-redis` 做一版可演进的后端骨架。

## 1. 请求链路

1. `cmd/api/main.go` 启动 Gin。
2. 加载 `configs/config.example.yaml`。
3. 初始化 MySQL（GORM）与 Redis。
4. 创建 `repository -> service -> api handler`。
5. `api` 层复用 `oapi-codegen` 生成的路由契约，挂载到 Gin。

## 2. 目录职责

- `cmd/api/main.go`: 示例入口，组装依赖。
- `internal/framework/config/loader.go`: YAML 配置加载。
- `internal/framework/orm/gorm.go`: GORM 初始化与连接池配置。
- `internal/framework/cache/redis_client.go`: Redis 客户端初始化。
- `internal/storage/model/*.go`: GORM 数据模型。
- `internal/storage/repository/*.go`: 持久化实现。
- `internal/storage/service/gorm_service.go`: 业务编排 + Redis 缓存。
- `internal/storage/api/*.go`: OpenAPI 接口实现与 Gin 路由挂载。

## 3. 你可以重点模仿的工程实践

1. 契约优先：接口定义改 `openapi.yaml`，生成代码后再实现。
2. 分层清晰：handler 不直接操作 DB，统一走 service/repository。
3. 依赖注入：main 里显式组装，不依赖全局变量。
4. 缓存策略：读取走缓存，写操作后删除缓存键。
5. 多租户边界：示例里以 `currentUserID` 占位，实际项目换成鉴权中间件注入。

## 4. 运行方式

1. 启动 MySQL 和 Redis（按 `configs/config.example.yaml`）。
2. 执行 `go test ./...` 验证代码。
3. 执行 `go run ./cmd/api` 启动服务。
4. 访问 `GET /healthz` 验证服务在线。

Redis 说明：

- `redis.required: true`：Redis 初始化失败直接退出（生产推荐）。
- `redis.required: false`：Redis 初始化失败时降级为无缓存模式（本地开发方便）。

## 5. 后续你可以做的增强

1. 将 `currentUserID` 替换为 JWT 中间件注入。
2. 为 `storage_setting` 增加 JSON 语义去重约束。
3. 把 Redis key 从全局改成按用户维度（如 `storage:platforms:active:{userID}`）。
4. 给 repository/service 增加单元测试和集成测试。
