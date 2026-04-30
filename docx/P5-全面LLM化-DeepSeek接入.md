# P5 全面LLM化（DeepSeek）

## 目标
- Agent 主链路全面切换到 LLM 决策，不再走规则路由。

## 改动
- 新增 `internal/agent/llm`：
  - `provider.go`：Provider 抽象
  - `deepseek_provider.go`：DeepSeek Chat API 实现
- `AgentService` 改为：
  - `LLM DecideTools -> 白名单校验 -> Tool执行 -> LLM Summarize`
- 响应新增：`routeMode/provider/model`
- 配置新增：`llm.provider/base_url/api_key/model/timeout_ms`
- 前端 Agent 面板展示模型信息。

## 安全边界
- LLM 仅做决策与总结，工具执行仍受 Registry 白名单约束。
- 非白名单工具名直接拒绝。

## 验证
- `go test ./...` 通过
- `frontend npm test` 通过
- `frontend npm run build` 通过
