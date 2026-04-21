# storage 业务线目录说明

## 目录职责

- `api/`
  - 对外 HTTP 协议适配层。
  - 包含 OpenAPI 生成代码（`storage.gen.go`）、路由挂载（`gin_routes.go`）、handler（`handler.go`）。
  - 负责 OpenAPI 类型与 service DTO 的双向转换。

- `service/`
  - 业务编排层。
  - `service.go` 定义接口，`types.go` 定义内部 DTO，`gorm_service.go` 实现业务规则（单活、语义配置、缓存策略）。
  - 不依赖 `api` 包，避免循环依赖。

- `repository/`
  - 数据访问层。
  - 只负责 GORM 查询与事务写入，不承载业务规则。

- `model/`
  - 持久化模型层（GORM Model）。
  - 与数据库表结构对齐。
  - `generate.go` 预留模型生成逻辑入口。

- `module/`
  - 模块装配层。
  - 实现 `app.Module`，负责把 storage 的 model、service、route 注册进系统。

## 依赖方向

`api -> service -> repository -> model`

`module` 只做装配，不承载业务逻辑。

## 请求主链路

1. 请求进入 `api/generate` 生成路由。
2. `handler` 解析 OpenAPI 请求并转换为 service 输入 DTO。
3. `service` 执行业务规则并调用 `repository`。
4. `repository` 读写 `model` 对应的数据表。
5. `handler` 将 service 输出 DTO 转回 OpenAPI 响应。
