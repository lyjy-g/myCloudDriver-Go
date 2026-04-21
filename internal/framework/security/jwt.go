package security

// TokenService 表示 token 相关能力占位。
type TokenService interface {
	Parse(token string) (string, error)
}
