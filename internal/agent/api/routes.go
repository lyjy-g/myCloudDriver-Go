package api

import (
	"net/http"

	agentapi "myclouddrive-go/internal/agent/api/gen"
	agentsvc "myclouddrive-go/internal/agent/service"
)

// RegisterRoutes 注册 Agent 路由（基于 OpenAPI 生成的路由器）。
func RegisterRoutes(mux *http.ServeMux, svc *agentsvc.AgentService) {
	h := NewHandler(svc)
	agentapi.HandlerFromMux(h, mux)
}
