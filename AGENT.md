# MyCloudDrive Agent Rules

## 1. 总原则（按你当前要求固化）
- 以后端为准：前端无条件适配后端接口与字段标准。
- 真实环境优先：禁止演示/种子数据（例如默认 `readme.txt`）。
- 保持中文优良注释：关键链路、状态机、幂等点必须可讲清。

## 2. Git 规则（执行版）
- 每次开始改动前先执行 `git status --short`，识别本次改动边界。
- 提交信息统一用 Conventional Commits：`feat|fix|refactor|docs|test|chore(scope): 中文说明`。
- 一次提交只做一件事：接口变更、前端联调、文档整理尽量分开提交。
- 禁止提交运行时目录和临时文件（如 `.data/`、IDE 缓存、日志临时文件）。
- 提交前至少做一次可运行校验：后端 `go build ./cmd/api`，前端 `npm --prefix frontend run build`（涉及对应模块时）。
- 除非明确要求，不执行 `git reset --hard`、不覆盖用户已有未提交改动。

## 3. File 模块当前实现约束
- `file_info` / `file_transfer_task` 必须真实参与业务链路，不允许只在内存中运转。
- 启动时：
  - 只初始化根目录；
  - 从 DB 回灌 `file_info` 与活跃传输任务（uploading/paused）。
- 写路径（创建目录、重命名、移动、上传合并、任务状态变更）必须同步持久化。

## 4. 幂等与状态机规则
- 请求幂等：`Idempotency-Key`，Redis 优先、内存兜底。
- 分片幂等：`taskId + chunkIndex` 去重，重复分片直接成功返回。
- 合并幂等：Redis 锁 + 状态二次校验（仅 `uploading` 可 merge）。
- 状态机白名单：
  - `uploading -> paused`
  - `paused -> uploading`
  - `uploading/paused -> canceled`
  - `completed` 不可再取消

## 6. 数据一致性与报错策略
- 关键落库失败要返回业务错误，不能吞掉。
- `content_md5` 字段仅接收合法 32 位 MD5；非 MD5（如 64 位 SHA-256）不入该列。
- 事务用于关键原子链路（如 merge 后任务完成 + 文件元数据写入）。

## 7. 日志与可观测性
- 请求日志保持结构化：method/path/status/duration + req/resp。
- 落库失败必须有明确错误日志，包含 taskId/fileId 便于排查。
