package api

import (
	"net/http"
	"strings"

	agentmodel "myclouddrive-go/internal/agent/model"
	agentsvc "myclouddrive-go/internal/agent/service"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/sse"
	"myclouddrive-go/internal/framework/web"
)

// Handler 提供 Agent 检索 API。
type Handler struct {
	svc *agentsvc.AgentService
}

func NewHandler(svc *agentsvc.AgentService) *Handler {
	return &Handler{svc: svc}
}

// Query 执行检索型 Agent 查询。
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.svc == nil {
		web.WriteJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "msg": "agent service unavailable", "data": nil})
		return
	}
	var req agentmodel.QueryRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "msg": "invalid request body", "data": nil})
		return
	}
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
	resp, err := h.svc.Query(r.Context(), req)
	if err != nil {
		switch {
		case code.Is(err, code.BadRequest):
			web.WriteJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "msg": err.Error(), "data": nil})
		case code.Is(err, code.NoPermission):
			web.WriteJSON(w, http.StatusForbidden, map[string]any{"code": 403, "msg": err.Error(), "data": nil})
		default:
			web.WriteJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "msg": err.Error(), "data": nil})
		}
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": resp})
}

// StreamQuery 流式 Agent 查询（SSE）。
func (h *Handler) StreamQuery(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.svc == nil {
		web.WriteJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "msg": "agent service unavailable", "data": nil})
		return
	}
	var req agentmodel.QueryRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "msg": "invalid request body", "data": nil})
		return
	}
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
	ch, cancel := sse.NewResponseWriter(w)
	if ch == nil {
		web.WriteJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "msg": "streaming not supported", "data": nil})
		return
	}
	defer cancel()

	sse.SendTo(ch, sse.Event{Event: "start", Data: map[string]any{"mode": req.Mode, "query": req.Query}})

	eventFn := func(event string, data any) {
		sse.SendTo(ch, sse.Event{Event: event, Data: data})
	}
	h.svc.StreamQuery(r.Context(), req, eventFn)

	sse.SendTo(ch, sse.Event{Event: "done", Data: map[string]any{"status": "ok"}})
}
