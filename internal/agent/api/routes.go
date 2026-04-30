package api

import (
	"net/http"

	agentsvc "myclouddrive-go/internal/agent/service"
)

// RegisterRoutes 注册 Agent 路由。
func RegisterRoutes(mux *http.ServeMux, svc *agentsvc.AgentService) {
	h := NewHandler(svc)
	mux.HandleFunc("POST /apis/agent/query", h.Query)
}
