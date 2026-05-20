# myclouddrive-go

MyCloudDrive 的 Go 实现，包含云盘核心能力与 Agent/RAG 能力。

## 核心能力

- 文件域：目录管理、文件列表、回收站、下载
- 传输域：分片上传（check/chunk/merge）、断点续传、秒传能力
- 存储域：多存储配置（Local/S3/MinIO）与按工作空间隔离
- 分享域：创建分享、提取码校验、公开访问、下载、访问记录
- Agent 域：检索/受控执行/RAG 查询、知识库管理与文件导入链路

## 当前模块

`cmd/api/main.go` 已注册以下模块：

1. `plugin`
2. `storage`
3. `user`
4. `file`
5. `share`
6. `agent`

## 快速启动

### 1) 启动依赖（MySQL/Redis/MinIO）

```bash
cd deploy
docker compose up -d
```

数据库初始化脚本：`deploy/mysql/init.sql`

### 2) 准备配置

```bash
cp configs/config.example.yaml configs/config.yaml
```

按需修改：

- `database.dsn`
- `redis.addr`
- `llm`（DeepSeek 等）
- `embedding`（向量模型配置）

> 安全说明：`configs/config.yaml` 已在 `.gitignore` 中，勿提交真实 API Key。

### 3) 启动后端

```bash
go run ./cmd/api
```

默认监听：`http://localhost:8080`

### 4) 启动前端（可选）

```bash
cd frontend
npm install
npm run dev
```

默认前端：`http://localhost:5173`

## API 组织方式

当前后端 API 已统一为 Gin 风格：

- 各模块路由集中在 `internal/{module}/api/routes.go`
- 各模块 handler 集中在 `internal/{module}/api/handler.go`
- 请求/响应 DTO 尽量放在各模块 `model` 下，避免 service 依赖生成代码

常用命令：

```bash
make test
make run-gin
```

## 关键目录

- `cmd/api`：服务入口
- `internal/app`：模块装配与启动
- `internal/framework`：配置、DB、Redis、HTTP 基础设施
- `internal/{storage,user,file,share,agent,plugin}`：业务模块
- `deploy`：本地依赖与数据库初始化
- `configs`：配置模板
- `frontend`：前端项目
