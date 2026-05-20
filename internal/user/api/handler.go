package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/framework/code"
	usermodel "myclouddrive-go/internal/user/model"
	"myclouddrive-go/internal/user/service"
)

const (
	successCodeStr   = "200"
	successMessage   = "success"
	badRequestCode   = 400
	noPermissionCode = 403
	notFoundCode     = 404
	internalCode     = 500
)

// Handler 处理 user 模块 HTTP 请求。
type Handler struct {
	svc *service.UserService
}

// NewHandler 创建 user 模块的 HTTP 处理器。
func NewHandler(svc *service.UserService) *Handler {
	return &Handler{svc: svc}
}

// DoLogin 处理用户名密码登录，并返回当前用户的登录态信息。
func (h *Handler) DoLogin(c *gin.Context) {
	var req usermodel.LoginCmd
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": badRequestCode, "message": "invalid request body"})
		return
	}
	result, err := h.svc.Login(c.Request.Context(), req, c.Request)
	if err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage, "data": result})
}

// Logout 处理当前登录用户登出。
func (h *Handler) Logout(c *gin.Context) {
	if err := h.svc.Logout(c.Request.Context()); err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage})
}

// GetDetail 返回当前登录用户的资料详情。
func (h *Handler) GetDetail(c *gin.Context) {
	user, err := h.svc.CurrentUser(c.Request.Context())
	if err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage, "data": user})
}

// Register 注册新用户，并初始化默认个人空间等基础数据。
func (h *Handler) Register(c *gin.Context) {
	var req usermodel.UserRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": badRequestCode, "message": "invalid request body"})
		return
	}
	if err := h.svc.Register(c.Request.Context(), req); err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage})
}

// EditUserInfo 更新当前登录用户的基础资料。
func (h *Handler) EditUserInfo(c *gin.Context) {
	var req usermodel.UserEditInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": badRequestCode, "message": "invalid request body"})
		return
	}
	if err := h.svc.UpdateUserInfo(c.Request.Context(), req); err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage})
}

// ResetPassword 处理登录态下的修改密码请求。
func (h *Handler) ResetPassword(c *gin.Context) {
	var req usermodel.PasswordEditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": badRequestCode, "message": "invalid request body"})
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), req); err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage})
}

// CheckForgetPasswordCode 校验忘记密码验证码并执行重置密码。
func (h *Handler) CheckForgetPasswordCode(c *gin.Context) {
	var req usermodel.PasswordForgetEditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": badRequestCode, "message": "invalid request body"})
		return
	}
	if err := h.svc.ResetForgetPassword(c.Request.Context(), req); err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage})
}

// SendForgetPasswordCodeByMail 发送找回密码验证码到指定邮箱。
func (h *Handler) SendForgetPasswordCodeByMail(c *gin.Context) {
	if err := h.svc.SendForgetPasswordCode(c.Request.Context(), strings.TrimSpace(c.Param("mail"))); err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage})
}

// GetUserTransferSetting 查询当前用户的传输设置。
func (h *Handler) GetUserTransferSetting(c *gin.Context) {
	item, err := h.svc.GetTransferSetting(c.Request.Context())
	if err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage, "data": item})
}

// UpdateUserTransferSetting 更新当前用户的传输设置。
func (h *Handler) UpdateUserTransferSetting(c *gin.Context) {
	var req usermodel.UserTransferSettingEditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": badRequestCode, "message": "invalid request body"})
		return
	}
	item, err := h.svc.UpdateTransferSetting(c.Request.Context(), req)
	if err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": successMessage, "data": item})
}

// ListUserWorkspaces 列出当前用户可访问的工作空间。
func (h *Handler) ListUserWorkspaces(c *gin.Context) {
	items, err := h.svc.ListWorkspaces(c.Request.Context())
	if err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": successCodeStr, "message": successMessage, "data": items})
}

// CreateOrgWorkspace 创建新的组织工作空间。
func (h *Handler) CreateOrgWorkspace(c *gin.Context) {
	var req usermodel.CreateOrgWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": badRequestCode, "message": "invalid request body"})
		return
	}
	item, err := h.svc.CreateOrgWorkspace(c.Request.Context(), req.Name)
	if err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": successCodeStr, "message": successMessage, "data": item})
}

// SetDefaultWorkspace 设置当前用户的默认工作空间。
func (h *Handler) SetDefaultWorkspace(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	if err := h.svc.SetDefaultWorkspace(c.Request.Context(), workspaceID); err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": successCodeStr, "message": successMessage, "data": gin.H{"defaultWorkspaceId": workspaceID}})
}

// ListWorkspaceMembers 查询指定工作空间的成员列表。
func (h *Handler) ListWorkspaceMembers(c *gin.Context) {
	items, err := h.svc.ListWorkspaceMembers(c.Request.Context(), c.Param("workspaceId"))
	if err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": successCodeStr, "message": successMessage, "data": items})
}

// AddWorkspaceMember 向指定工作空间新增成员。
func (h *Handler) AddWorkspaceMember(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	var req usermodel.AddWorkspaceMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": badRequestCode, "message": "invalid request body"})
		return
	}
	if err := h.svc.AddWorkspaceMember(c.Request.Context(), workspaceID, req.UserId, req.Role); err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": successCodeStr, "message": successMessage, "data": gin.H{"workspaceId": workspaceID, "userId": req.UserId, "role": req.Role}})
}

// RemoveWorkspaceMember 从指定工作空间移除成员。
func (h *Handler) RemoveWorkspaceMember(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	userID := c.Param("userId")
	if err := h.svc.RemoveWorkspaceMember(c.Request.Context(), workspaceID, userID); err != nil {
		writeUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": successCodeStr, "message": successMessage, "data": gin.H{"workspaceId": workspaceID, "userId": userID}})
}

func writeUserError(c *gin.Context, err error) {
	switch {
	case code.Is(err, code.BadRequest):
		c.JSON(http.StatusBadRequest, gin.H{"code": badRequestCode, "message": err.Error()})
	case code.Is(err, code.NoPermission):
		c.JSON(http.StatusForbidden, gin.H{"code": noPermissionCode, "message": err.Error()})
	case code.Is(err, code.NotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": notFoundCode, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": internalCode, "message": err.Error()})
	}
}
