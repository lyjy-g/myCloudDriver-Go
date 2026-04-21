package api

import (
	"net/http"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/web"
	gen "myclouddrive-go/internal/user/api/gen"
	"myclouddrive-go/internal/user/service"
)

// Handler 基于 OpenAPI 生成的 ServerInterface 实现 user 接口。
type Handler struct {
	svc *service.PlaceholderService
}

func NewHandler(svc *service.PlaceholderService) *Handler {
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
