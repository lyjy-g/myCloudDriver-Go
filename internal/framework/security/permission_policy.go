package security

import (
	"context"
	"strings"

	"myclouddrive-go/internal/framework/code"
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
