package utils

import (
	"context"
	"fmt"
	"strings"

	"myclouddrive-go/internal/framework/security"
)

// ContextBuilder 构建 Agent 执行所需的上下文。
type ContextBuilder struct{}

func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{}
}

// BuildContext 从请求上下文提取 agent 依赖信息。
func (b *ContextBuilder) BuildContext(ctx context.Context) (userID, workspaceID string, err error) {
	principal, ok := security.GetCtxInfo(ctx)
	if !ok {
		return "", "", fmt.Errorf("login required")
	}
	userID = strings.TrimSpace(principal.UserID)
	workspaceID = strings.TrimSpace(principal.WorkspaceID)
	if userID == "" {
		return "", "", fmt.Errorf("login required")
	}
	return userID, workspaceID, nil
}
