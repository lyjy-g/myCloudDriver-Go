package api

import (
	"net/http"
	"strings"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/web"
	gen "myclouddrive-go/internal/share/api/gen"
	"myclouddrive-go/internal/share/service"
)

// Handler 基于 OpenAPI 生成与手工扩展路由实现 share 接口。
type Handler struct {
	svc *service.ShareService
}

func NewHandler(svc *service.ShareService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) PingShare(w http.ResponseWriter, r *http.Request) {
	msg, err := h.svc.Ping(r.Context())
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, gen.PingResponse{Code: "OK", Message: msg})
}

func (h *Handler) CreateShare(w http.ResponseWriter, r *http.Request) {
	var req service.CreateShareReq
	if err := web.DecodeJSON(r, &req); err != nil {
		writeShareError(w, code.New(code.BadRequest, "invalid request body"))
		return
	}
	item, err := h.svc.CreateShare(r.Context(), req)
	if err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

func (h *Handler) ListMyShares(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListMyShares(r.Context())
	if err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(items))
}

func (h *Handler) GetShareDetail(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("shareId")
	item, err := h.svc.GetShareDetail(r.Context(), shareID, true)
	if err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

func (h *Handler) UpdateShare(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("shareId")
	var req service.UpdateShareReq
	if err := web.DecodeJSON(r, &req); err != nil {
		writeShareError(w, code.New(code.BadRequest, "invalid request body"))
		return
	}
	item, err := h.svc.UpdateShare(r.Context(), shareID, req)
	if err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

func (h *Handler) AccessPublicShare(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("shareId")
	var req struct {
		ShareCode string `json:"shareCode"`
	}
	if err := web.DecodeJSON(r, &req); err != nil {
		writeShareError(w, code.New(code.BadRequest, "invalid request body"))
		return
	}
	item, err := h.svc.PublicAccess(r.Context(), shareID, req.ShareCode, r)
	if err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

func (h *Handler) VerifyShareCode(w http.ResponseWriter, r *http.Request) {
	var req service.VerifyShareCodeReq
	if err := web.DecodeJSON(r, &req); err != nil {
		writeShareError(w, code.New(code.BadRequest, "invalid request body"))
		return
	}
	okValue, err := h.svc.VerifyShareCode(r.Context(), req.ShareID, req.ShareCode)
	if err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(okValue))
}

func (h *Handler) GetShareInfo(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("shareId")
	item, err := h.svc.GetShareInfo(r.Context(), shareID)
	if err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

func (h *Handler) GetShareItems(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("shareId")
	items, err := h.svc.GetShareItems(r.Context(), shareID, r.URL.Query().Get("parentId"))
	if err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(items))
}

func (h *Handler) DownloadShareFile(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("shareId")
	fileID := r.PathValue("fileId")
	shareCode := strings.TrimSpace(r.Header.Get("X-Share-Code"))
	if shareCode == "" {
		shareCode = strings.TrimSpace(r.URL.Query().Get("shareCode"))
	}
	content, fileName, err := h.svc.DownloadShareFile(r.Context(), shareID, fileID, shareCode, r)
	if err != nil {
		writeShareError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *Handler) GetAccessRecords(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("shareId")
	items, err := h.svc.ListAccessRecords(r.Context(), shareID)
	if err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(items))
}

func (h *Handler) CancelShares(w http.ResponseWriter, r *http.Request) {
	var shareIDs []string
	if err := web.DecodeJSON(r, &shareIDs); err != nil {
		writeShareError(w, code.New(code.BadRequest, "invalid request body"))
		return
	}
	if err := h.svc.CancelShares(r.Context(), shareIDs); err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

func (h *Handler) CancelAllShares(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.CancelAllShares(r.Context()); err != nil {
		writeShareError(w, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(nil))
}

func ok(data any) map[string]any {
	return map[string]any{"code": 200, "msg": "success", "data": data}
}

func writeShareError(w http.ResponseWriter, err error) {
	switch {
	case code.Is(err, code.BadRequest):
		web.WriteJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "msg": err.Error(), "data": nil})
	case code.Is(err, code.NotFound):
		web.WriteJSON(w, http.StatusNotFound, map[string]any{"code": 404, "msg": err.Error(), "data": nil})
	case code.Is(err, code.NoPermission):
		web.WriteJSON(w, http.StatusUnauthorized, map[string]any{"code": 401, "msg": err.Error(), "data": nil})
	default:
		web.WriteJSON(w, http.StatusInternalServerError, map[string]any{"code": 500, "msg": err.Error(), "data": nil})
	}
}
