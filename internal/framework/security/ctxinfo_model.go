package security

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

// ctxInfoKey 使用私有空结构体作为 context key，避免与其他包 key 冲突。
// 可忽略：key 的具体类型不重要，核心是“私有且不可碰撞”。
type ctxInfoKey struct{}
