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

// Claims 表示 token 载荷。
type Claims struct {
	UserID      string `json:"sub"`
	WorkspaceID string `json:"ws,omitempty"`
	Username    string `json:"un,omitempty"`
	IssuedAt    int64  `json:"iat"`
	ExpireAt    int64  `json:"exp"`
}

// JWTService 提供 HS256 token 的签发与解析能力。
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

// Issue 签发 token。
func (s *JWTService) Issue(userID, workspaceID, username string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", errors.New("user id is required")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	now := time.Now().Unix()
	claims := Claims{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Username:    username,
		IssuedAt:    now,
		ExpireAt:    now + int64(ttl.Seconds()),
	}

	headerRaw, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	claimsRaw, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims failed: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerRaw)
	payload := base64.RawURLEncoding.EncodeToString(claimsRaw)
	signText := header + "." + payload
	signature := s.sign(signText)

	return signText + "." + signature, nil
}

// Parse 解析并校验 token。
func (s *JWTService) Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token format")
	}

	signText := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(s.sign(signText)), []byte(parts[2])) {
		return Claims{}, errors.New("invalid token signature")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errors.New("invalid token payload")
	}

	var claims Claims
	if err = json.Unmarshal(payloadRaw, &claims); err != nil {
		return Claims{}, errors.New("invalid token claims")
	}
	if strings.TrimSpace(claims.UserID) == "" {
		return Claims{}, errors.New("invalid token subject")
	}
	if claims.ExpireAt <= time.Now().Unix() {
		return Claims{}, errors.New("token expired")
	}
	return claims, nil
}

func (s *JWTService) sign(text string) string {
	h := hmac.New(sha256.New, s.secret)
	_, _ = h.Write([]byte(text))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
