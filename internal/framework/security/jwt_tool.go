package security

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// GetToken 签发 token。
//
// 1. 最小必填是 userID，避免匿名 token 进入系统。
// 2. ttl 合法性兜底，避免调用方传 0 造成“签发即过期”。
// 3. header.payload.signature 三段式，符合 JWT 标准格式。
//
// - header 是固定 HS256+JWT，通常不会频繁变更。
func (s *JWTService) GetToken(userID, workspaceID, username string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", errors.New("user id is required")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	now := time.Now().Unix()
	tokenInfo := TokenInfo{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Username:    username,
		StartAt:     now,
		ExpireAt:    now + int64(ttl.Seconds()),
	}

	headerRaw, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})

	bodyRaw, err := json.Marshal(tokenInfo)
	if err != nil {
		return "", fmt.Errorf("marshal claims failed: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerRaw)
	payload := base64.RawURLEncoding.EncodeToString(bodyRaw)
	signText := header + "." + payload
	signature := s.sign(signText)

	return signText + "." + signature, nil
}

// ParseToken 解析并校验 token。
//
// 1. 先校验 token 三段格式；
// 2. 再校验签名完整性（防篡改）；
// 3. 再反序列化 claims；
// 4. 最后校验业务字段与过期时间。
func (s *JWTService) ParseToken(token string) (TokenInfo, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return TokenInfo{}, errors.New("invalid token format")
	}

	signText := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(s.sign(signText)), []byte(parts[2])) {
		return TokenInfo{}, errors.New("invalid token signature")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return TokenInfo{}, errors.New("invalid token payload")
	}

	var claims TokenInfo
	if err = json.Unmarshal(payloadRaw, &claims); err != nil {
		return TokenInfo{}, errors.New("invalid token claims")
	}
	if strings.TrimSpace(claims.UserID) == "" {
		return TokenInfo{}, errors.New("invalid token subject")
	}
	if claims.ExpireAt <= time.Now().Unix() {
		return TokenInfo{}, errors.New("token expired")
	}
	return claims, nil
}

func (s *JWTService) sign(text string) string {
	// 可忽略：sign 是纯函数封装，便于单测和复用，不影响外部协议。
	h := hmac.New(sha256.New, s.secret)
	_, _ = h.Write([]byte(text))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
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
