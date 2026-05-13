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

// 权限点命名规范：module.resource.action
// 统一放在这里，避免散落在各个 handler 中难以维护。
const (
	PermissionFileRead         = "file.read"
	PermissionFileWrite        = "file.write"
	PermissionFileTransferRead = "file.transfer.read"
	PermissionFileTransferExec = "file.transfer.exec"

	PermissionStoragePlatformRead = "storage.platform.read"
	PermissionStorageSettingRead  = "storage.setting.read"
	PermissionStorageSettingWrite = "storage.setting.write"
)

// permissionMinRole 定义每个接口权限点对应的最小角色要求。
//
// 角色等级：owner > admin > member > viewer。
// 若后续要新增更细粒度控制（如 share/audit），只需要在这里扩展即可。
var permissionMinRole = map[string]string{
	PermissionFileRead:         RoleMember,
	PermissionFileWrite:        RoleMember,
	PermissionFileTransferRead: RoleMember,
	PermissionFileTransferExec: RoleMember,

	PermissionStoragePlatformRead: RoleMember,
	PermissionStorageSettingRead:  RoleMember,
	PermissionStorageSettingWrite: RoleAdmin,
}

// RequirePermission 按权限点执行统一授权检查。
func RequirePermission(ctx context.Context, permission string) (CtxInfo, error) {
	minRole, ok := permissionMinRole[permission]
	if !ok {
		return CtxInfo{}, code.New(code.NoPermission, "permission not configured: "+permission)
	}
	return RequireWorkspaceRoleAtLeast(ctx, minRole)
}

// MinRoleOfPermission 返回权限点对应的最小角色（用于调试/文档生成）。
func MinRoleOfPermission(permission string) string {
	return strings.TrimSpace(permissionMinRole[permission])
}

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
