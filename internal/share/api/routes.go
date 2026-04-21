package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/share/service"
)

// RegisterRoutes 注册 share 路由（占位）。
func RegisterRoutes(r *gin.Engine, svc service.Service) {
	g := r.Group("/api/v1/share")
	g.GET("/ping", func(c *gin.Context) {
		msg, err := svc.Ping(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": "OK", "message": msg})
	})
}
