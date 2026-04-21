package api

import (
	"net/http"

	gen "myclouddrive-go/internal/share/api/gen"
	"myclouddrive-go/internal/share/service"
)

// RegisterRoutes 将 OpenAPI 生成路由注册到标准库 ServeMux。
func RegisterRoutes(mux *http.ServeMux, svc *service.PlaceholderService) {
	h := NewHandler(svc)
	gen.HandlerFromMux(h, mux)
}
