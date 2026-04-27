package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// AuthMiddleware 解析 Bearer Token，并将 CtxInfo 写入请求上下文。
//
// 1. 非侵入：无 token 或 token 非法时不直接中断，由业务层决定是否要求登录；
// 2. 统一：仅在入口解析一次，后续 handler/service 统一从 context 读取身份；
// 3. 可扩展：支持 Redis 黑名单，实现“登出后 token 立即失效”。
func AuthMiddleware(jwtSvc *JWTService, rdb redis.Cmdable) func(http.Handler) http.Handler {
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

// isBlacklisted 判断 token 是否已进入黑名单。
//
// 面试可讲：
// - 这是 JWT 无状态认证常见补偿机制，解决“服务端主动失效 token”问题。
// 可忽略：
// - 这里将 Redis miss 统一视作未拉黑，属于简化策略。
func isBlacklisted(ctx context.Context, rdb redis.Cmdable, token string) bool {
	if rdb == nil {
		return false
	}
	key := "auth:blacklist:" + tokenHash(token)
	_, err := rdb.Get(ctx, key).Result()
	return err == nil
}

// BlacklistToken 将 token 拉黑到过期时间，支持登出立即失效。
//
// 面试可讲：
// 1. 黑名单 TTL 对齐 token 过期时间，避免永久脏数据；
// 2. exp 已过期时给一个短 TTL（5 分钟）兜底，防止时间边界抖动。
func BlacklistToken(ctx context.Context, rdb redis.Cmdable, token string, expUnix int64) error {
	if rdb == nil || strings.TrimSpace(token) == "" {
		return nil
	}
	ttl := time.Until(time.Unix(expUnix, 0))
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	key := "auth:blacklist:" + tokenHash(token)
	return rdb.Set(ctx, key, "1", ttl).Err()
}

// tokenHash 对原始 token 做哈希后再作为缓存 key。
//
// 面试可讲：
// - 避免在缓存层直接暴露原始 token（降低泄漏敏感度）。
// 可忽略：
// - SHA-256 本身不是鉴权关键点，这里只用于 key 脱敏。
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
