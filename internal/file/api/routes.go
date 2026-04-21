package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/file/service"
)

// RegisterRoutes 注册 file 路由（占位）。
func RegisterRoutes(r *gin.Engine, svc service.Service) {
	g := r.Group("/api/v1/file")
	g.GET("/ping", func(c *gin.Context) {
		msg, err := svc.Ping(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": "OK", "message": msg})
	})
}
