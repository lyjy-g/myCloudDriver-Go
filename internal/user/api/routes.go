package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/user/service"
)

// RegisterRoutes 注册 user 路由（占位）。
func RegisterRoutes(r *gin.Engine, svc service.Service) {
	g := r.Group("/api/v1/user")
	g.GET("/ping", func(c *gin.Context) {
		msg, err := svc.Ping(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": "OK", "message": msg})
	})
}
