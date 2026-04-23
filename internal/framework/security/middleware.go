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

// AuthMiddleware 解析 Bearer Token，并将主体写入 context。
func AuthMiddleware(jwtSvc *JWTService, rdb redis.Cmdable) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if jwtSvc == nil {
				next.ServeHTTP(w, r)
				return
			}
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				next.ServeHTTP(w, r)
				return
			}
			token := strings.TrimSpace(authz[len("Bearer "):])
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			if isBlacklisted(r.Context(), rdb, token) {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := jwtSvc.Parse(token)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := WithPrincipal(r.Context(), Principal{
				UserID:      claims.UserID,
				Username:    claims.Username,
				WorkspaceID: claims.WorkspaceID,
				Token:       token,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isBlacklisted(ctx context.Context, rdb redis.Cmdable, token string) bool {
	if rdb == nil {
		return false
	}
	key := "auth:blacklist:" + tokenHash(token)
	_, err := rdb.Get(ctx, key).Result()
	return err == nil
}

// BlacklistToken 将 token 拉黑到过期时间，支持登出立即失效。
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

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
