package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TokenInfo 表示 JWT 的业务载荷（Claims）。
//
// 1. sub（UserID）是“谁在访问”的主身份，必须存在。
// 2. ws（WorkspaceID）是“在哪个租户空间访问”，用于多租户隔离。
// 3. iat/exp 是“何时签发/何时过期”，用于时效控制。
type TokenInfo struct {
	UserID      string `json:"sub"`
	WorkspaceID string `json:"ws,omitempty"`
	Username    string `json:"un,omitempty"`
	StartAt     int64  `json:"iat"`
	ExpireAt    int64  `json:"exp"`
}

// JWTService 提供最小可用的 HS256 JWT 签发与验签能力。
//
// - 这里使用对称签名（HMAC-SHA256），服务端单点签发/校验，简单高效。
// - 若系统升级为多服务、多环境强隔离，可演进到 KMS 管理密钥或非对称签名。
type JWTService struct {
	secret []byte
}

// NewJWTService 创建 JWT 服务。
func NewJWTService(secret string) *JWTService {
	if strings.TrimSpace(secret) == "" {
		secret = "myclouddrive-go-dev-secret"
	}
	return &JWTService{secret: []byte(secret)}
}

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
