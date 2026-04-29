package api

import (
	"net/http"
	"strings"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/web"
	userapi "myclouddrive-go/internal/user/api/gen"
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

// Handler 基于 OpenAPI 生成与手工扩展路由实现 user 接口。
type Handler struct {
	svc *service.UserService
}

func NewHandler(svc *service.UserService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) DoLogin(w http.ResponseWriter, r *http.Request, _ userapi.DoLoginParams) {
	var req userapi.LoginCmd
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, badRequestCode, "invalid request body")
		return
	}
	result, err := h.svc.Login(r.Context(), req, r)
	if err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage), Data: result})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context()); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage)})
}

func (h *Handler) GetDetail(w http.ResponseWriter, r *http.Request) {
	user, err := h.svc.CurrentUser(r.Context())
	if err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultSysUserResponse{
		Code: int32Ptr(200),
		Msg:  strPtr(successMessage),
		Data: user,
	})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req userapi.UserRegisterRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, badRequestCode, "invalid request body")
		return
	}
	if err := h.svc.Register(r.Context(), req); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage)})
}

func (h *Handler) EditUserInfo(w http.ResponseWriter, r *http.Request) {
	var req userapi.UserEditInfoRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, badRequestCode, "invalid request body")
		return
	}
	if err := h.svc.UpdateUserInfo(r.Context(), req); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage)})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req userapi.PasswordEditRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, badRequestCode, "invalid request body")
		return
	}
	if err := h.svc.ChangePassword(r.Context(), req); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage)})
}

func (h *Handler) CheckForgetPasswordCode(w http.ResponseWriter, r *http.Request) {
	var req userapi.PasswordForgetEditRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, badRequestCode, "invalid request body")
		return
	}
	if err := h.svc.ResetForgetPassword(r.Context(), req); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage)})
}

func (h *Handler) SendForgetPasswordCodeByMail(w http.ResponseWriter, r *http.Request, mail string) {
	if err := h.svc.SendForgetPasswordCode(r.Context(), strings.TrimSpace(mail)); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage)})
}

func (h *Handler) GetUserTransferSetting(w http.ResponseWriter, r *http.Request) {
	item, err := h.svc.GetTransferSetting(r.Context())
	if err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultSysUserTransferSetting{
		Code: int32Ptr(200),
		Msg:  strPtr(successMessage),
		Data: item,
	})
}

func (h *Handler) UpdateUserTransferSetting(w http.ResponseWriter, r *http.Request) {
	var req userapi.UserTransferSettingEditRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, badRequestCode, "invalid request body")
		return
	}
	item, err := h.svc.UpdateTransferSetting(r.Context(), req)
	if err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultSysUserTransferSetting{
		Code: int32Ptr(200),
		Msg:  strPtr(successMessage),
		Data: item,
	})
}

func (h *Handler) ListUserWorkspaces(w http.ResponseWriter, r *http.Request, _ userapi.ListUserWorkspacesParams) {
	items, err := h.svc.ListWorkspaces(r.Context())
	if err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": successCodeStr, "message": successMessage, "data": items})
}

func (h *Handler) CreateOrgWorkspace(w http.ResponseWriter, r *http.Request) {
	var req userapi.CreateOrgWorkspaceJSONBody
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, badRequestCode, "invalid request body")
		return
	}
	item, err := h.svc.CreateOrgWorkspace(r.Context(), req.Name)
	if err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": successCodeStr, "message": successMessage, "data": item})
}

func (h *Handler) SetDefaultWorkspace(w http.ResponseWriter, r *http.Request, workspaceID string) {
	if err := h.svc.SetDefaultWorkspace(r.Context(), workspaceID); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{
		"code":    successCodeStr,
		"message": successMessage,
		"data": map[string]string{
			"defaultWorkspaceId": workspaceID,
		},
	})
}

func (h *Handler) ListWorkspaceMembers(w http.ResponseWriter, r *http.Request, workspaceID string) {
	items, err := h.svc.ListWorkspaceMembers(r.Context(), workspaceID)
	if err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{"code": successCodeStr, "message": successMessage, "data": items})
}

func (h *Handler) AddWorkspaceMember(w http.ResponseWriter, r *http.Request, workspaceID string) {
	var req userapi.AddWorkspaceMemberJSONBody
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, badRequestCode, "invalid request body")
		return
	}
	if err := h.svc.AddWorkspaceMember(r.Context(), workspaceID, req.UserId, string(req.Role)); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{
		"code":    successCodeStr,
		"message": successMessage,
		"data": map[string]string{
			"workspaceId": workspaceID,
			"userId":      req.UserId,
			"role":        string(req.Role),
		},
	})
}

func (h *Handler) RemoveWorkspaceMember(w http.ResponseWriter, r *http.Request, workspaceID string, userID string) {
	if err := h.svc.RemoveWorkspaceMember(r.Context(), workspaceID, userID); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]any{
		"code":    successCodeStr,
		"message": successMessage,
		"data": map[string]string{
			"workspaceId": workspaceID,
			"userId":      userID,
		},
	})
}

// 兼容现有前端路径。
func (h *Handler) ListWorkspacesCompat(w http.ResponseWriter, r *http.Request) {
	h.ListUserWorkspaces(w, r, userapi.ListUserWorkspacesParams{})
}

// 兼容现有前端路径。
func (h *Handler) GetActiveWorkspaceCompat(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListWorkspaces(r.Context())
	if err != nil {
		writeUserError(w, err)
		return
	}
	for _, item := range items {
		if item.IsDefault {
			web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage), Data: item})
			return
		}
	}
	if len(items) == 0 {
		web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage)})
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultObject{Code: int32Ptr(200), Msg: strPtr(successMessage), Data: items[0]})
}

// 兼容现有前端路径。
func (h *Handler) SetActiveWorkspaceCompat(w http.ResponseWriter, r *http.Request) {
	var req usermodel.ActiveWorkspaceCmd
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, badRequestCode, "invalid request body")
		return
	}
	if err := h.svc.SetDefaultWorkspace(r.Context(), req.WorkspaceID); err != nil {
		writeUserError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, userapi.ResultObject{
		Code: int32Ptr(200),
		Msg:  strPtr(successMessage),
		Data: map[string]string{"defaultWorkspaceId": req.WorkspaceID},
	})
}

func writeUserError(w http.ResponseWriter, err error) {
	switch {
	case code.Is(err, code.BadRequest):
		web.WriteError(w, http.StatusBadRequest, badRequestCode, err.Error())
	case code.Is(err, code.NoPermission):
		web.WriteError(w, http.StatusForbidden, noPermissionCode, err.Error())
	case code.Is(err, code.NotFound):
		web.WriteError(w, http.StatusNotFound, notFoundCode, err.Error())
	default:
		web.WriteError(w, http.StatusInternalServerError, internalCode, err.Error())
	}
}

func strPtr(v string) *string {
	return &v
}

func int32Ptr(v int32) *int32 {
	return &v
}
