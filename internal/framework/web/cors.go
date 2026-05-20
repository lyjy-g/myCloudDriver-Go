package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSOptions 定义 CORS 中间件配置。
type CORSOptions struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAgeSeconds    int
}

// DefaultCORSOptions 返回默认 CORS 配置。
func DefaultCORSOptions() CORSOptions {
	return CORSOptions{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Requested-With", "Idempotency-Key", "X-Workspace-Id", "X-Storage-Setting-Id"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type", "Content-Disposition", "X-Idempotent-Replayed"},
		AllowCredentials: false,
		MaxAgeSeconds:    600,
	}
}

// GinCORSMiddleware 提供 Gin 版本的 CORS 处理。
//
// 这里统一处理两类事情：
// 1. 给真实请求补上跨域响应头；
// 2. 对浏览器 OPTIONS 预检请求直接短路返回。
//
// 现在项目的 HTTP 入口已经统一为 Gin，所以这里只保留 Gin 版本。
func GinCORSMiddleware(options CORSOptions) gin.HandlerFunc {
	allowOrigins := options.AllowOrigins
	allowMethods := options.AllowMethods
	allowHeaders := options.AllowHeaders

	methodsValue := strings.Join(allowMethods, ", ")
	headersValue := strings.Join(allowHeaders, ", ")
	exposeValue := strings.Join(options.ExposeHeaders, ", ")
	maxAgeValue := "600"
	if options.MaxAgeSeconds > 0 {
		maxAgeValue = strconv.Itoa(options.MaxAgeSeconds)
	}

	allowAllOrigins := len(allowOrigins) == 1 && allowOrigins[0] == "*"
	originSet := make(map[string]struct{}, len(allowOrigins))
	for _, item := range allowOrigins {
		originSet[item] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if allowAllOrigins {
				c.Header("Access-Control-Allow-Origin", "*")
			} else if _, ok := originSet[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			}
		}
		if options.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Allow-Methods", methodsValue)
		c.Header("Access-Control-Allow-Headers", headersValue)
		if strings.TrimSpace(exposeValue) != "" {
			c.Header("Access-Control-Expose-Headers", exposeValue)
		}
		c.Header("Access-Control-Max-Age", maxAgeValue)

		// 预检请求不进入业务 handler，直接在网关层短路返回。
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}
