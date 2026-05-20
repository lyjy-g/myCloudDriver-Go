package api

import (
	"github.com/gin-gonic/gin"

	agentsvc "myclouddrive-go/internal/agent/service"
)

// RegisterRoutes 注册 Agent 路由。
// 这里按“查询 -> 执行记录 -> 会话 -> 知识库 -> 工作流 -> 工具调用历史”分段排列。
func RegisterRoutes(router gin.IRouter, svc *agentsvc.AgentService) {
	h := NewHandler(svc)

	// 同步 Agent 查询。
	router.POST("/apis/agent/query", h.AgentQuery)
	// 流式 Agent 查询。
	router.POST("/apis/agent/stream", h.AgentStreamQuery)
	// 停止流式查询。
	router.POST("/apis/agent/stop/:traceId", h.StopStreamQuery)
	// 查询执行记录列表。
	router.GET("/apis/agent/actions", h.ListAgentActions)
	// 查询单次执行记录详情。
	router.GET("/apis/agent/action/:traceId", h.GetAgentAction)
	// 确认执行计划。
	router.POST("/apis/agent/confirm/:traceId", h.ConfirmAction)

	// 创建会话。
	router.POST("/apis/agent/session", h.CreateSession)
	// 删除会话。
	router.DELETE("/apis/agent/session/:sessionId", h.DeleteSession)
	// 查询会话列表。
	router.GET("/apis/agent/sessions", h.ListSessions)

	// 查询知识库列表。
	router.GET("/apis/agent/knowledge", h.ListKnowledge)
	// 创建知识库。
	router.POST("/apis/agent/knowledge", h.CreateKnowledge)
	// 查询单个知识库。
	router.GET("/apis/agent/knowledge/:kbId", h.GetKnowledge)
	// 删除知识库。
	router.DELETE("/apis/agent/knowledge/:kbId", h.DeleteKnowledge)
	// 查询知识库文件列表。
	router.GET("/apis/agent/knowledge/:kbId/files", h.ListKnowledgeFiles)
	// 添加知识库文件。
	router.POST("/apis/agent/knowledge/:kbId/file", h.AddKnowledgeFile)
	// 删除知识库文件。
	router.DELETE("/apis/agent/knowledge/:kbId/file/:fileId", h.RemoveKnowledgeFile)

	// 查询工作流列表。
	router.GET("/apis/agent/workflows", h.ListWorkflows)
	// 保存工作流定义。
	router.POST("/apis/agent/workflow", h.SaveWorkflow)
	// 查询工作流详情。
	router.GET("/apis/agent/workflow/:wfId", h.GetWorkflow)
	// 删除工作流定义。
	router.DELETE("/apis/agent/workflow/:wfId", h.DeleteWorkflow)
	// 查询工作流运行详情。
	router.GET("/apis/agent/workflow/run/:wfRunId", h.GetWorkflowRun)
	// 触发工作流 webhook。
	router.POST("/apis/agent/workflow/webhook", h.TriggerWorkflowWebhook)
	// 查询工具调用历史。
	router.GET("/apis/agent/tool-calls", h.ListToolCalls)
	// 查询对话历史。
	router.GET("/apis/agent/history", h.ListHistory)
}
