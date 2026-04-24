package security

import "context"

// Principal 表示请求级登录主体（认证后身份快照）。
//
// 面试可讲：
// - 这是“认证结果在请求链路中的载体”，避免每层重复解析 token。
// - 包含 UserID + WorkspaceID，天然支持“用户身份 + 租户空间”双维度鉴权。
type Principal struct {
	UserID      string
	Username    string
	WorkspaceID string
	Token       string
}

// principalKey 使用私有空结构体作为 context key，避免与其他包 key 冲突。
// 可忽略：key 的具体类型不重要，核心是“私有且不可碰撞”。
type principalKey struct{}

// WithPrincipal 将登录主体写入上下文。
//
// - context 只传“请求生命周期内的元信息”，不传可变业务状态。
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext 从上下文读取登录主体。
//
// 面试可讲：
// - 返回 (Principal, bool) 而不是 panic，符合 Go 显式错误处理风格。
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
