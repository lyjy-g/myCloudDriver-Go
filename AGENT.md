# 模块模型使用规则

在 `internal/业务模块` 开发中统一遵循以下约束：

1. `handler` 和 `service` 中使用到的 OpenAPI 请求/返回结构，必须优先使用 `internal/xx/api/gen` 生成类型。
2. `service` 中使用到的用户域数据库模型，必须优先使用 `internal/xx/model/generator_main.go` 生成产物（`internal/user/model/dbmodel`）。
3. 非 OpenAPI/非 DB 生成的业务 DTO，统一定义在 `internal/xx/model/dto.go`，禁止分散到 `handler`/`service` 文件内重复定义。
4. 新增接口或字段时，先更新 OpenAPI 与 model 生成入口，再同步代码使用，避免手写结构体与生成代码漂移。

