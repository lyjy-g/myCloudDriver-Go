package api

import (
	"net/http"
	"strings"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/web"
	gen "myclouddrive-go/internal/user/api/gen"
	"myclouddrive-go/internal/user/service"
)

// Handler 基于 OpenAPI 生成与手工扩展路由实现 user 接口。
type Handler struct {
	svc *service.UserService
}

func NewHandler(svc *service.UserService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) PingUser(w http.ResponseWriter, r *http.Request) {
	msg, err := h.svc.Ping(r.Context())
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, gen.PingResponse{Code: "OK", Message: msg})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req service.LoginReq
	if err := web.DecodeJSON(r, &req); err != nil {
		writeResultError(w, http.StatusBadRequest, code.BadRequest, "invalid request body")
		return
	}
	result, err := h.svc.Login(r.Context(), req, r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(result))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context()); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.svc.CurrentUser(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(user))
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req service.RegisterReq
	if err := web.DecodeJSON(r, &req); err != nil {
		writeResultError(w, http.StatusBadRequest, code.BadRequest, "invalid request body")
		return
	}
	if err := h.svc.Register(r.Context(), req); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

func (h *Handler) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	h.Me(w, r)
}

func (h *Handler) UpdateUserInfo(w http.ResponseWriter, r *http.Request) {
	var req service.UpdateUserReq
	if err := web.DecodeJSON(r, &req); err != nil {
		writeResultError(w, http.StatusBadRequest, code.BadRequest, "invalid request body")
		return
	}
	if err := h.svc.UpdateUserInfo(r.Context(), req); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req service.ChangePasswordReq
	if err := web.DecodeJSON(r, &req); err != nil {
		writeResultError(w, http.StatusBadRequest, code.BadRequest, "invalid request body")
		return
	}
	if err := h.svc.ChangePassword(r.Context(), req); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

func (h *Handler) SendForgetPasswordCode(w http.ResponseWriter, r *http.Request) {
	mail := strings.TrimSpace(r.PathValue("mail"))
	if err := h.svc.SendForgetPasswordCode(r.Context(), mail); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

func (h *Handler) ResetForgetPassword(w http.ResponseWriter, r *http.Request) {
	var req service.ForgetPasswordReq
	if err := web.DecodeJSON(r, &req); err != nil {
		writeResultError(w, http.StatusBadRequest, code.BadRequest, "invalid request body")
		return
	}
	if err := h.svc.ResetForgetPassword(r.Context(), req); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

func (h *Handler) GetTransferSetting(w http.ResponseWriter, r *http.Request) {
	item, err := h.svc.GetTransferSetting(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

func (h *Handler) UpdateTransferSetting(w http.ResponseWriter, r *http.Request) {
	var req service.TransferSettingInput
	if err := web.DecodeJSON(r, &req); err != nil {
		writeResultError(w, http.StatusBadRequest, code.BadRequest, "invalid request body")
		return
	}
	item, err := h.svc.UpdateTransferSetting(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

func (h *Handler) ListUserWorkspaces(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListWorkspaces(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(items))
}

func (h *Handler) CreateOrgWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := web.DecodeJSON(r, &req); err != nil {
		writeResultError(w, http.StatusBadRequest, code.BadRequest, "invalid request body")
		return
	}
	item, err := h.svc.CreateOrgWorkspace(r.Context(), req.Name)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

func (h *Handler) SetDefaultWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if err := h.svc.SetDefaultWorkspace(r.Context(), workspaceID); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(map[string]string{"defaultWorkspaceId": workspaceID}))
}

func (h *Handler) ListWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	items, err := h.svc.ListWorkspaceMembers(r.Context(), workspaceID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(items))
}

func (h *Handler) AddWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	var req struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := web.DecodeJSON(r, &req); err != nil {
		writeResultError(w, http.StatusBadRequest, code.BadRequest, "invalid request body")
		return
	}
	if err := h.svc.AddWorkspaceMember(r.Context(), workspaceID, req.UserID, req.Role); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

func (h *Handler) RemoveWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	targetUserID := r.PathValue("userId")
	if err := h.svc.RemoveWorkspaceMember(r.Context(), workspaceID, targetUserID); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

// 兼容现有前端路径。
func (h *Handler) ListWorkspacesCompat(w http.ResponseWriter, r *http.Request) {
	h.ListUserWorkspaces(w, r)
}

func (h *Handler) GetActiveWorkspaceCompat(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListWorkspaces(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	for _, item := range items {
		if item.IsDefault {
			web.WriteJSON(w, http.StatusOK, ok(item))
			return
		}
	}
	if len(items) == 0 {
		web.WriteJSON(w, http.StatusOK, ok(nil))
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(items[0]))
}

func (h *Handler) SetActiveWorkspaceCompat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID string `json:"workspaceId"`
	}
	if err := web.DecodeJSON(r, &req); err != nil {
		writeResultError(w, http.StatusBadRequest, code.BadRequest, "invalid request body")
		return
	}
	if err := h.svc.SetDefaultWorkspace(r.Context(), req.WorkspaceID); err != nil {
		writeServiceError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(map[string]string{"defaultWorkspaceId": req.WorkspaceID}))
}

func ok(data any) map[string]any {
	return map[string]any{"code": 200, "msg": "success", "data": data}
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case code.Is(err, code.BadRequest):
		writeResultError(w, http.StatusBadRequest, code.BadRequest, err.Error())
	case code.Is(err, code.NotFound):
		writeResultError(w, http.StatusNotFound, code.NotFound, err.Error())
	case code.Is(err, code.NoPermission):
		writeResultError(w, http.StatusUnauthorized, code.NoPermission, err.Error())
	default:
		writeResultError(w, http.StatusInternalServerError, code.InternalError, err.Error())
	}
}

func writeResultError(w http.ResponseWriter, status int, c code.Code, msg string) {
	web.WriteJSON(w, status, map[string]any{"code": int32(status), "msg": msg, "errorCode": string(c), "data": nil})
}
