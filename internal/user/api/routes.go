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

	// 认证
	mux.HandleFunc("POST /apis/auth/login", h.Login)
	mux.HandleFunc("POST /apis/auth/logout", h.Logout)
	mux.HandleFunc("GET /apis/auth/me", h.Me)

	// 用户
	mux.HandleFunc("POST /apis/user/register", h.Register)
	mux.HandleFunc("GET /apis/user/info", h.GetUserInfo)
	mux.HandleFunc("PUT /apis/user/info", h.UpdateUserInfo)
	mux.HandleFunc("PUT /apis/user/password", h.ChangePassword)
	mux.HandleFunc("GET /apis/user/forget-password/code/{mail}", h.SendForgetPasswordCode)
	mux.HandleFunc("PUT /apis/user/forget-password", h.ResetForgetPassword)
	mux.HandleFunc("GET /apis/user/transfer/setting", h.GetTransferSetting)
	mux.HandleFunc("PUT /apis/user/transfer/setting", h.UpdateTransferSetting)

	// 工作空间（主路径）
	mux.HandleFunc("GET /apis/user/workspaces", h.ListUserWorkspaces)
	mux.HandleFunc("POST /apis/user/workspaces", h.CreateOrgWorkspace)
	mux.HandleFunc("PUT /apis/user/default-workspace/{workspaceId}", h.SetDefaultWorkspace)
	mux.HandleFunc("GET /apis/user/workspaces/{workspaceId}/members", h.ListWorkspaceMembers)
	mux.HandleFunc("POST /apis/user/workspaces/{workspaceId}/members", h.AddWorkspaceMember)
	mux.HandleFunc("DELETE /apis/user/workspaces/{workspaceId}/members/{userId}", h.RemoveWorkspaceMember)

	// 兼容前端历史路径
	mux.HandleFunc("GET /apis/workspaces", h.ListWorkspacesCompat)
	mux.HandleFunc("GET /apis/workspaces/active", h.GetActiveWorkspaceCompat)
	mux.HandleFunc("POST /apis/workspaces/active", h.SetActiveWorkspaceCompat)
}
