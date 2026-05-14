package security

import (
	"strings"
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
