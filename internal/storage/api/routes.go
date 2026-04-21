package api

import (
	"myclouddrive-go/internal/storage/api/gen"

	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/storage/service"
)

// RegisterGinRoutes 将 OpenAPI 生成路由挂到 Gin。
// 这里复用生成的 net/http handler，避免手写路径参数绑定。
func RegisterGinRoutes(r *gin.Engine, svc *service.StorageService) {
	h := NewHandler(svc)
	std := gen.Handler(h)

	// 兼容 /api/v1/storage 与其子路径。
	r.Any("/api/v1/storage", gin.WrapH(std))
	r.Any("/api/v1/storage/*any", gin.WrapH(std))
}
