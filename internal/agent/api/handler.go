package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	agentapi "myclouddrive-go/internal/agent/api/gen"
	agentmodel "myclouddrive-go/internal/agent/model"
	agentsvc "myclouddrive-go/internal/agent/service"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	"myclouddrive-go/internal/framework/sse"
	"myclouddrive-go/internal/framework/web"
)

// Handler 实现 agentapi.ServerInterface，处理 Agent HTTP 请求。
type Handler struct {
	svc *agentsvc.AgentService
}

func NewHandler(svc *agentsvc.AgentService) *Handler {
	return &Handler{svc: svc}
}

var _ agentapi.ServerInterface = (*Handler)(nil)

// ============================================================
// Agent 查询
// ============================================================

// AgentQuery 同步 Agent 查询（search / rag 模式）。
func (h *Handler) AgentQuery(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	var req agentmodel.QueryRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	fillRequestFromHeader(&req, r)
	resp, err := h.svc.Query(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": resp})
}

// AgentStreamQuery 流式 Agent 查询（SSE）。
func (h *Handler) AgentStreamQuery(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	var req agentmodel.QueryRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	fillRequestFromHeader(&req, r)

	ch, cancel := sse.NewResponseWriter(w)
	if ch == nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	defer cancel()

	eventFn := func(event string, data any) {
		sse.SendTo(ch, sse.Event{Event: event, Data: data})
	}
	h.svc.StreamQuery(r.Context(), req, eventFn)
}

// ============================================================
// 执行确认（execute 模式）
// ============================================================

func (h *Handler) ConfirmAction(w http.ResponseWriter, r *http.Request, traceId string, params agentapi.ConfirmActionParams) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	resp, err := h.svc.ConfirmExecute(r.Context(), strings.TrimSpace(traceId))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{
		"code": 200,
		"msg":  "success",
		"data": resp,
	})
}

// ============================================================
// 停止流式查询
// ============================================================

func (h *Handler) StopStreamQuery(w http.ResponseWriter, r *http.Request, traceId string) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	err := h.svc.StopStream(r.Context(), traceId)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "stopped", "data": map[string]any{"traceId": traceId, "partial": true}})
}

// ============================================================
// 执行记录
// ============================================================

func (h *Handler) ListAgentActions(w http.ResponseWriter, r *http.Request, params agentapi.ListAgentActionsParams) {
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": map[string]any{"total": 0, "items": []any{}, "page": 1, "size": 20}})
}

func (h *Handler) GetAgentAction(w http.ResponseWriter, r *http.Request, traceId string) {
	writeError(w, http.StatusNotFound, "action not found: "+traceId)
}

// ============================================================
// 会话管理
// ============================================================

func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "session not implemented")
}

func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request, sessionId string) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": []any{}})
}

// ============================================================
// 知识库管理（RAG Agent）
// ============================================================

func (h *Handler) ListKnowledge(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	items, err := h.svc.ListKnowledgeByWorkspace(r.Context(), principal.WorkspaceID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": items})
}

func (h *Handler) CreateKnowledge(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	var req agentapi.CreateKnowledgeRequest
	if err = web.DecodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := h.svc.CreateKnowledge(
		r.Context(),
		principal.WorkspaceID,
		principal.UserID,
		req.Name,
		strings.TrimSpace(func() string {
			if req.Description == nil {
				return ""
			}
			return *req.Description
		}()),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": created})
}

func (h *Handler) GetKnowledge(w http.ResponseWriter, r *http.Request, kbId string) {
	writeError(w, http.StatusNotFound, "knowledge not found: "+kbId)
}

func (h *Handler) DeleteKnowledge(w http.ResponseWriter, r *http.Request, kbId string) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	numericID, err := strconv.ParseInt(strings.TrimSpace(kbId), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid knowledge id")
		return
	}
	if err = h.svc.DeleteKnowledge(r.Context(), principal.WorkspaceID, numericID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListKnowledgeFiles(w http.ResponseWriter, r *http.Request, kbId string) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	knowledgeID, err := strconv.ParseInt(strings.TrimSpace(kbId), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid knowledge id")
		return
	}
	items, err := h.svc.ListKnowledgeFiles(r.Context(), principal.WorkspaceID, knowledgeID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": items})
}

func (h *Handler) AddKnowledgeFile(w http.ResponseWriter, r *http.Request, kbId string) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	principal, err := security.RequireLogin(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	knowledgeID, err := strconv.ParseInt(strings.TrimSpace(kbId), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid knowledge id")
		return
	}
	var req agentapi.AddKnowledgeFileRequest
	if err = web.DecodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	storageSettingID := strings.TrimSpace(r.Header.Get("X-Storage-Setting-Id"))
	if req.StorageSettingId != nil && strings.TrimSpace(*req.StorageSettingId) != "" {
		storageSettingID = strings.TrimSpace(*req.StorageSettingId)
	}
	item, err := h.svc.AddKnowledgeFile(r.Context(), principal.WorkspaceID, knowledgeID, req.FileId, storageSettingID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": item})
}

func (h *Handler) RemoveKnowledgeFile(w http.ResponseWriter, r *http.Request, kbId string, fileId string) {
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// 工作流管理（Workflow Agent）
// ============================================================

func (h *Handler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": []any{}})
}

func (h *Handler) SaveWorkflow(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "workflow save not implemented")
}

func (h *Handler) GetWorkflow(w http.ResponseWriter, r *http.Request, wfId string) {
	writeError(w, http.StatusNotFound, "workflow not found: "+wfId)
}

func (h *Handler) DeleteWorkflow(w http.ResponseWriter, r *http.Request, wfId string) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetWorkflowRun(w http.ResponseWriter, r *http.Request, wfRunId string) {
	writeError(w, http.StatusNotFound, "workflow run not found: "+wfRunId)
}

func (h *Handler) TriggerWorkflowWebhook(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "workflow webhook not implemented")
}

// ============================================================
// 工具调用历史
// ============================================================

func (h *Handler) ListToolCalls(w http.ResponseWriter, r *http.Request, params agentapi.ListToolCallsParams) {
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": map[string]any{"total": 0, "items": []any{}, "page": 1, "size": 20}})
}

// ============================================================
// 对话历史
// ============================================================

// ListHistory 获取对话历史（OpenAPI 生成接口）。
func (h *Handler) ListHistory(w http.ResponseWriter, r *http.Request, params agentapi.ListHistoryParams) {
	if h == nil || h.svc == nil {
		writeError(w, http.StatusInternalServerError, "agent service unavailable")
		return
	}
	n := 10
	if params.Size != nil && *params.Size > 0 {
		n = *params.Size
	}
	beforeTraceID := ""
	if params.Before != nil {
		beforeTraceID = *params.Before
	}
	principal, err := security.RequireLogin(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	entries, hasMore, err := h.svc.ListHistory(r.Context(), principal.UserID, principal.WorkspaceID, beforeTraceID, n)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{
		"code": 200, "msg": "success",
		"data": map[string]any{
			"items":   entries,
			"hasMore": hasMore,
			"before":  beforeTraceID,
		},
	})
}

// ============================================================
// 辅助方法
// ============================================================

func fillRequestFromHeader(req *agentmodel.QueryRequest, r *http.Request) {
	if strings.TrimSpace(req.WorkspaceID) == "" {
		req.WorkspaceID = strings.TrimSpace(r.Header.Get("X-Workspace-Id"))
	}
	if strings.TrimSpace(req.StorageSettingID) == "" {
		req.StorageSettingID = strings.TrimSpace(r.Header.Get("X-Storage-Setting-Id"))
	}
	if strings.TrimSpace(req.Scope) == "" {
		req.Scope = "auto"
	}
	if strings.TrimSpace(req.Mode) == "" {
		req.Mode = "search"
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	web.WriteJSON(w, status, map[string]any{"code": status, "msg": msg, "data": nil})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case code.Is(err, code.BadRequest):
		web.WriteJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "msg": err.Error(), "data": nil})
	case code.Is(err, code.NotFound):
		web.WriteJSON(w, http.StatusNotFound, map[string]any{"code": 404, "msg": err.Error(), "data": nil})
	case code.Is(err, code.NoPermission):
		web.WriteJSON(w, http.StatusForbidden, map[string]any{"code": 403, "msg": err.Error(), "data": nil})
	default:
		web.WriteJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "msg": err.Error(), "data": nil})
	}
}

// 确保导入不被移除
var _ = json.Marshal
