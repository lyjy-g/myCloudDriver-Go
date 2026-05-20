package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	agentmodel "myclouddrive-go/internal/agent/model"
	agentsvc "myclouddrive-go/internal/agent/service"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	"myclouddrive-go/internal/framework/sse"
)

type Handler struct {
	svc *agentsvc.AgentService
}

// NewHandler 创建 agent 模块的 HTTP 处理器。
func NewHandler(svc *agentsvc.AgentService) *Handler {
	return &Handler{svc: svc}
}

// AgentQuery 处理同步 Agent 查询，请求完成后一次性返回结果。
func (h *Handler) AgentQuery(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	var req agentmodel.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	fillRequestFromHeader(&req, c)
	resp, err := h.svc.Query(c.Request.Context(), req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": resp})
}

// AgentStreamQuery 处理流式 Agent 查询，并通过 SSE 持续推送过程事件。
func (h *Handler) AgentStreamQuery(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	var req agentmodel.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	fillRequestFromHeader(&req, c)

	ch, cancel := sse.NewResponseWriter(c.Writer)
	if ch == nil {
		writeError(c, http.StatusInternalServerError, "streaming not supported")
		return
	}
	defer cancel()

	eventFn := func(event string, data any) {
		sse.SendTo(ch, sse.Event{Event: event, Data: data})
	}
	h.svc.StreamQuery(c.Request.Context(), req, eventFn)
}

// ConfirmAction 确认某次高风险 Agent 执行计划并继续执行。
func (h *Handler) ConfirmAction(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	resp, err := h.svc.ConfirmExecute(c.Request.Context(), strings.TrimSpace(c.Param("traceId")))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": resp})
}

// StopStreamQuery 停止指定 trace 的流式 Agent 查询。
func (h *Handler) StopStreamQuery(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	if err := h.svc.StopStream(c.Request.Context(), c.Param("traceId")); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "stopped", "data": gin.H{"traceId": c.Param("traceId"), "partial": true}})
}

// ListAgentActions 返回 Agent 执行记录列表。
func (h *Handler) ListAgentActions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": gin.H{"total": 0, "items": []any{}, "page": 1, "size": 20}})
}

// GetAgentAction 返回单次 Agent 执行记录详情。
func (h *Handler) GetAgentAction(c *gin.Context) {
	writeError(c, http.StatusNotFound, "action not found: "+c.Param("traceId"))
}

// CreateSession 创建 Agent 会话。
func (h *Handler) CreateSession(c *gin.Context) {
	writeError(c, http.StatusNotImplemented, "session not implemented")
}

// DeleteSession 删除指定 Agent 会话。
func (h *Handler) DeleteSession(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// ListSessions 返回当前用户的 Agent 会话列表。
func (h *Handler) ListSessions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": []any{}})
}

// ListKnowledge 返回当前工作空间下的知识库列表。
func (h *Handler) ListKnowledge(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	items, err := h.svc.ListKnowledgeByWorkspace(c.Request.Context(), principal.WorkspaceID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": items})
}

// CreateKnowledge 创建新的知识库。
func (h *Handler) CreateKnowledge(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	var req agentmodel.CreateKnowledgeRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	description := ""
	if req.Description != nil {
		description = strings.TrimSpace(*req.Description)
	}
	created, err := h.svc.CreateKnowledge(c.Request.Context(), principal.WorkspaceID, principal.UserID, req.Name, description)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": created})
}

// GetKnowledge 返回单个知识库详情。
func (h *Handler) GetKnowledge(c *gin.Context) {
	writeError(c, http.StatusNotFound, "knowledge not found: "+c.Param("kbId"))
}

// DeleteKnowledge 删除指定知识库。
func (h *Handler) DeleteKnowledge(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	numericID, err := strconv.ParseInt(strings.TrimSpace(c.Param("kbId")), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid knowledge id")
		return
	}
	if err = h.svc.DeleteKnowledge(c.Request.Context(), principal.WorkspaceID, numericID); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListKnowledgeFiles 返回指定知识库下的文件导入状态列表。
func (h *Handler) ListKnowledgeFiles(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	knowledgeID, err := strconv.ParseInt(strings.TrimSpace(c.Param("kbId")), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid knowledge id")
		return
	}
	items, err := h.svc.ListKnowledgeFiles(c.Request.Context(), principal.WorkspaceID, knowledgeID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": items})
}

// AddKnowledgeFile 把文件加入知识库，并启动导入流程。
func (h *Handler) AddKnowledgeFile(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	knowledgeID, err := strconv.ParseInt(strings.TrimSpace(c.Param("kbId")), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid knowledge id")
		return
	}
	var req agentmodel.AddKnowledgeFileRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	storageSettingID := currentStorageSettingID(c)
	if req.StorageSettingId != nil && strings.TrimSpace(*req.StorageSettingId) != "" {
		storageSettingID = strings.TrimSpace(*req.StorageSettingId)
	}
	item, err := h.svc.AddKnowledgeFile(c.Request.Context(), principal.WorkspaceID, knowledgeID, req.FileId, storageSettingID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": item})
}

// RemoveKnowledgeFile 从知识库中移除指定文件。
func (h *Handler) RemoveKnowledgeFile(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	knowledgeID, err := strconv.ParseInt(strings.TrimSpace(c.Param("kbId")), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid knowledge id")
		return
	}
	if err = h.svc.RemoveKnowledgeFile(c.Request.Context(), principal.WorkspaceID, knowledgeID, c.Param("fileId")); err != nil {
		writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListWorkflows 返回工作流定义列表。
func (h *Handler) ListWorkflows(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": []any{}})
}

// SaveWorkflow 创建或更新工作流定义。
func (h *Handler) SaveWorkflow(c *gin.Context) {
	writeError(c, http.StatusNotImplemented, "workflow save not implemented")
}

// GetWorkflow 返回单个工作流定义详情。
func (h *Handler) GetWorkflow(c *gin.Context) {
	writeError(c, http.StatusNotFound, "workflow not found: "+c.Param("wfId"))
}

// DeleteWorkflow 删除指定工作流定义。
func (h *Handler) DeleteWorkflow(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// GetWorkflowRun 返回单次工作流运行详情。
func (h *Handler) GetWorkflowRun(c *gin.Context) {
	writeError(c, http.StatusNotFound, "workflow run not found: "+c.Param("wfRunId"))
}

// TriggerWorkflowWebhook 处理工作流 webhook 触发请求。
func (h *Handler) TriggerWorkflowWebhook(c *gin.Context) {
	writeError(c, http.StatusNotImplemented, "workflow webhook not implemented")
}

// ListToolCalls 返回 Agent 工具调用历史列表。
func (h *Handler) ListToolCalls(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": gin.H{"total": 0, "items": []any{}, "page": 1, "size": 20}})
}

// ListHistory 返回当前用户的 Agent 对话历史。
func (h *Handler) ListHistory(c *gin.Context) {
	if h == nil || h.svc == nil {
		writeError(c, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	n := 10
	if size, err := strconv.Atoi(strings.TrimSpace(c.Query("size"))); err == nil && size > 0 {
		n = size
	}
	beforeTraceID := strings.TrimSpace(c.Query("before"))
	principal, err := security.RequireLogin(c.Request.Context())
	if err != nil {
		writeServiceError(c, err)
		return
	}
	entries, hasMore, err := h.svc.ListHistory(c.Request.Context(), principal.UserID, principal.WorkspaceID, beforeTraceID, n)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200, "msg": "success",
		"data": gin.H{"items": entries, "hasMore": hasMore, "before": beforeTraceID},
	})
}

// fillRequestFromHeader 把 Gin 请求头里的上下文补进 Agent 查询 DTO。
// 这样前端可以少传一部分字段，而 Agent 仍然能拿到完整的工作空间/存储作用域。
func fillRequestFromHeader(req *agentmodel.QueryRequest, c *gin.Context) {
	if strings.TrimSpace(req.WorkspaceID) == "" {
		req.WorkspaceID = strings.TrimSpace(c.GetHeader("X-Workspace-Id"))
	}
	if strings.TrimSpace(req.StorageSettingID) == "" {
		req.StorageSettingID = currentStorageSettingID(c)
	}
	if strings.TrimSpace(req.Scope) == "" {
		req.Scope = "auto"
	}
	if strings.TrimSpace(req.Mode) == "" {
		req.Mode = "search"
	}
}

func writeError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "msg": msg, "data": nil})
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case code.Is(err, code.BadRequest):
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "data": nil})
	case code.Is(err, code.NotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error(), "data": nil})
	case code.Is(err, code.NoPermission):
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": err.Error(), "data": nil})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
	}
}

// currentStorageSettingID 和 file 模块保持同一套读取规则，避免 Agent 和文件业务口径不一致。
func currentStorageSettingID(c *gin.Context) string {
	if settingID := strings.TrimSpace(c.GetHeader("X-Storage-Setting-Id")); settingID != "" {
		return settingID
	}
	if principal, ok := security.GetCtxInfo(c.Request.Context()); ok {
		return strings.TrimSpace(principal.CurrentStorageSettingID)
	}
	return ""
}
