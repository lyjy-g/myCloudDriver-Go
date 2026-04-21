package api

import (
	"net/http"

	gen "myclouddrive-go/internal/storage/api/gen"
	"myclouddrive-go/internal/storage/service"
)

// RegisterRoutes 将 OpenAPI 生成路由注册到标准库 ServeMux。
func RegisterRoutes(mux *http.ServeMux, svc *service.StorageService) {
	h := NewHandler(svc)
	gen.HandlerFromMux(h, mux)
}
