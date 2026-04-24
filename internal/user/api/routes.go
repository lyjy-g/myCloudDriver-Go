package api

import (
	"net/http"

	gen "myclouddrive-go/internal/user/api/gen"
	"myclouddrive-go/internal/user/service"
)

// RegisterRoutes 将 OpenAPI 生成路由与业务路由注册到标准库 ServeMux。
func RegisterRoutes(mux *http.ServeMux, svc *service.UserService) {
	h := NewHandler(svc)
	gen.HandlerFromMux(h, mux)
	// 兼容历史前端工作空间路径。
	mux.HandleFunc("GET /apis/workspaces", h.ListWorkspacesCompat)
	mux.HandleFunc("GET /apis/workspaces/active", h.GetActiveWorkspaceCompat)
	mux.HandleFunc("POST /apis/workspaces/active", h.SetActiveWorkspaceCompat)
}
