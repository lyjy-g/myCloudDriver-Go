package security

import (
	"context"
	"strings"
)

// CtxInfo 表示请求级登录主体（认证后身份快照）。
//
// 面试可讲：
// - 这是“认证结果在请求链路中的载体”，避免每层重复解析 token。
// - 包含 UserID + WorkspaceID，天然支持“用户身份 + 租户空间”双维度鉴权。
type CtxInfo struct {
	UserID                  string
	Username                string
	WorkspaceID             string
	WorkspaceRole           string
	CurrentStorageSettingID string
	Token                   string
}

// CurrentStorageSettingCacheKey 返回“用户在工作空间下当前选中的存储配置”缓存 key。
func CurrentStorageSettingCacheKey(userID, workspaceID string) string {
	return "storage:selected:" + strings.TrimSpace(workspaceID) + ":" + strings.TrimSpace(userID)
}

// ctxInfoKey 使用私有空结构体作为 context key，避免与其他包 key 冲突。
// 可忽略：key 的具体类型不重要，核心是“私有且不可碰撞”。
type ctxInfoKey struct{}

// PutCtxInfo 将登录主体写入上下文。
//
// - context 只传“请求生命周期内的元信息”，不传可变业务状态。
func PutCtxInfo(ctx context.Context, p CtxInfo) context.Context {
	return context.WithValue(ctx, ctxInfoKey{}, p)
}

// GetCtxInfo 从上下文读取登录主体。
//
// - 返回 (CtxInfo, bool) 而不是 panic，符合 Go 显式错误处理风格。
func GetCtxInfo(ctx context.Context) (CtxInfo, bool) {
	//判空
	if ctx == nil {
		return CtxInfo{}, false
	}
	p, ok := ctx.Value(ctxInfoKey{}).(CtxInfo)
	return p, ok
}
