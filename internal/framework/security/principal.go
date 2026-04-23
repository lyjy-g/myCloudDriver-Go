package security

import "context"

// Principal 表示登录主体信息。
type Principal struct {
	UserID      string
	Username    string
	WorkspaceID string
	Token       string
}

type principalKey struct{}

// WithPrincipal 将登录主体写入上下文。
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext 从上下文读取登录主体。
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
