package web

import (
	"net/http"
	"strconv"
	"strings"
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
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Requested-With", "Idempotency-Key", "X-Workspace-Id"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type", "X-Idempotent-Replayed"},
		AllowCredentials: false,
		MaxAgeSeconds:    600,
	}
}

// CORSMiddleware 提供通用 CORS 处理能力。
func CORSMiddleware(options CORSOptions) func(http.Handler) http.Handler {
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

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" {
				if allowAllOrigins {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := originSet[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
			}
			if options.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Access-Control-Allow-Methods", methodsValue)
			w.Header().Set("Access-Control-Allow-Headers", headersValue)
			if strings.TrimSpace(exposeValue) != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposeValue)
			}
			w.Header().Set("Access-Control-Max-Age", maxAgeValue)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
