package security

import (
	"net/http"
	"strings"

	"github.com/redis/go-redis/v9"
)

// JWTMiddleware 解析 Bearer Token，并将 CtxInfo 写入请求上下文。
//
// 1. 非侵入：无 token 或 token 非法时不直接中断，由业务层决定是否要求登录；
// 2. 统一：仅在入口解析一次，后续 handler/service 统一从 context 读取身份；
// 3. 可扩展：支持 Redis 黑名单，实现“登出后 token 立即失效”。
func JWTMiddleware(jwtSvc *JWTService, rdb redis.Cmdable) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			// jwtSvc 为空时直接透传，主要用于本地开发或测试场景。
			if jwtSvc == nil {
				next.ServeHTTP(responseWriter, request)
				return
			}
			authz := strings.TrimSpace(request.Header.Get("Authorization"))
			// 仅识别 Bearer 令牌，其它认证方案由上层网关或其他中间件处理。
			if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				next.ServeHTTP(responseWriter, request)
				return
			}
			token := strings.TrimSpace(authz[len("Bearer "):])
			if token == "" {
				next.ServeHTTP(responseWriter, request)
				return
			}

			if isBlacklisted(request.Context(), rdb, token) {
				// 黑名单命中时不注入 Principal，交给后续鉴权点按“未登录”处理。
				next.ServeHTTP(responseWriter, request)
				return
			}

			claims, err := jwtSvc.ParseToken(token)
			if err != nil {
				next.ServeHTTP(responseWriter, request)
				return
			}

			ctx := PutCtxInfo(request.Context(), CtxInfo{
				UserID:      claims.UserID,
				Username:    claims.Username,
				WorkspaceID: claims.WorkspaceID,
				Token:       token,
			})
			// 将认证结果沿请求链路透传，避免重复解析 JWT。
			next.ServeHTTP(responseWriter, request.WithContext(ctx))
		})
	}
}
