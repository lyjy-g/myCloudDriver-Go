package security

import (
	"context"
	"strings"

	"myclouddrive-go/internal/framework/code"
)

const (
	RoleViewer = "viewer"
	RoleMember = "member"
	RoleAdmin  = "admin"
	RoleOwner  = "owner"
)

// RequireLogin 校验登录态。
func RequireLogin(ctx context.Context) (CtxInfo, error) {
	p, ok := GetCtxInfo(ctx)
	if !ok || strings.TrimSpace(p.UserID) == "" {
		return CtxInfo{}, code.New(code.NoPermission, "login required")
	}
	return p, nil
}

// RequireWorkspaceRoleAtLeast 校验工作空间角色是否达到最小权限等级。
//
// 等级：owner > admin > member > viewer。
func RequireWorkspaceRoleAtLeast(ctx context.Context, minRole string) (CtxInfo, error) {
	p, err := RequireLogin(ctx)
	if err != nil {
		return CtxInfo{}, err
	}
	if roleLevel(strings.ToLower(strings.TrimSpace(p.WorkspaceRole))) < roleLevel(strings.ToLower(strings.TrimSpace(minRole))) {
		return CtxInfo{}, code.New(code.NoPermission, "workspace permission denied")
	}
	return p, nil
}

func roleLevel(role string) int {
	switch role {
	case RoleOwner:
		return 4
	case RoleAdmin:
		return 3
	case RoleMember:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}
