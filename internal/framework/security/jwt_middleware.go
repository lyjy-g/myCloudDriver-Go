package security

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// GinJWTMiddleware 解析 Bearer Token，并把认证结果写入 request context。
//
// 1. 非侵入：无 token 或 token 非法时不直接中断，由业务层决定是否要求登录；
// 2. 统一：仅在入口解析一次，后续 handler/service 统一从 context 读取身份；
// 3. 可扩展：支持 Redis 黑名单，实现“登出后 token 立即失效”。
// 这里故意保持“弱拦截”：
// - token 缺失或非法时不直接返回 401；
// - 只是尽量把已认证身份写进 context；
// - 真正要求登录的接口，再由 handler/service 主动调用 RequireLogin。
func GinJWTMiddleware(jwtSvc *JWTService, rdb redis.Cmdable) gin.HandlerFunc {
	return func(c *gin.Context) {
		if jwtSvc == nil {
			c.Next()
			return
		}
		authz := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			c.Next()
			return
		}
		token := strings.TrimSpace(authz[len("Bearer "):])
		if token == "" || isBlacklisted(c.Request.Context(), rdb, token) {
			c.Next()
			return
		}
		claims, err := jwtSvc.ParseToken(token)
		if err != nil {
			c.Next()
			return
		}

		// 认证成功后，把解析结果放回 request context。
		// 后面的 Gin handler、service 都统一从 context 拿，不重复解析 JWT。
		ctx := PutCtxInfo(c.Request.Context(), CtxInfo{
			UserID:      claims.UserID,
			Username:    claims.Username,
			WorkspaceID: claims.WorkspaceID,
			Token:       token,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
