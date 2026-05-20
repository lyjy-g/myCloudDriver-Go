package api

import (
	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/user/service"
)

// RegisterRoutes 注册 user 模块路由。
// 路由顺序按“认证 -> 用户资料 -> 工作空间”排列，后端只保留一套主路径。
func RegisterRoutes(router gin.IRouter, svc *service.UserService) {
	h := NewHandler(svc)

	// 登录并建立用户会话。
	router.POST("/apis/auth/login", h.DoLogin)
	// 登出并使当前 token 失效。
	router.POST("/apis/auth/logout", h.Logout)
	// 查询当前登录用户资料。
	router.GET("/apis/user/detail", h.GetDetail)
	// 注册新用户。
	router.POST("/apis/user/register", h.Register)
	// 修改当前用户资料。
	router.PUT("/apis/user/info", h.EditUserInfo)
	// 登录态修改密码。
	router.PUT("/apis/user/password", h.ResetPassword)
	// 校验忘记密码验证码并重置密码。
	router.POST("/apis/user/password/forget/check", h.CheckForgetPasswordCode)
	// 发送忘记密码验证码。
	router.POST("/apis/user/password/forget/send/:mail", h.SendForgetPasswordCodeByMail)
	// 查询当前用户的传输设置。
	router.GET("/apis/user/transfer-setting", h.GetUserTransferSetting)
	// 更新当前用户的传输设置。
	router.PUT("/apis/user/transfer-setting", h.UpdateUserTransferSetting)

	// 查询当前用户可访问的工作空间列表。
	router.GET("/apis/user/workspaces", h.ListUserWorkspaces)
	// 创建组织工作空间。
	router.POST("/apis/user/workspaces", h.CreateOrgWorkspace)
	// 设置当前用户默认工作空间。
	router.PUT("/apis/user/default-workspace/:workspaceId", h.SetDefaultWorkspace)
	// 查询工作空间成员列表。
	router.GET("/apis/user/workspaces/:workspaceId/members", h.ListWorkspaceMembers)
	// 向工作空间添加成员。
	router.POST("/apis/user/workspaces/:workspaceId/members", h.AddWorkspaceMember)
	// 从工作空间移除成员。
	router.DELETE("/apis/user/workspaces/:workspaceId/members/:userId", h.RemoveWorkspaceMember)
}
