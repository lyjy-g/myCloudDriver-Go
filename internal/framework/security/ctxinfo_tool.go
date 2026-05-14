package security

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"myclouddrive-go/internal/framework/code"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// CurrentStorageSettingCacheKey 返回“用户在工作空间下当前选中的存储配置”缓存 key。
func CurrentStorageSettingCacheKey(userID, workspaceID string) string {
	return "storage:selected:" + strings.TrimSpace(workspaceID) + ":" + strings.TrimSpace(userID)
}

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

func collectWorkspaceCandidates(r *http.Request, db *gorm.DB, userID string, requestedWorkspaceID string, tokenWorkspaceID string) []string {
	candidates := make([]string, 0, 4)
	push := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || slices.Contains(candidates, id) {
			return
		}
		candidates = append(candidates, id)
	}
	push(requestedWorkspaceID)
	push(tokenWorkspaceID)

	var defaultWorkspaceID string
	_ = db.WithContext(r.Context()).
		Table("user").
		Select("default_workspace_id").
		Where("id = ?", userID).
		Limit(1).
		Scan(&defaultWorkspaceID).Error
	push(defaultWorkspaceID)

	var firstWorkspaceID string
	_ = db.WithContext(r.Context()).
		Table("workspace_member").
		Select("workspace_id").
		Where("user_id = ? AND status = 1", userID).
		Order("joined_at asc").
		Limit(1).
		Scan(&firstWorkspaceID).Error
	push(firstWorkspaceID)

	return candidates
}

func resolveWorkspaceRole(r *http.Request, db *gorm.DB, userID, workspaceID string) (string, bool, error) {
	type memberRow struct {
		Role string `gorm:"column:role"`
	}
	var row memberRow
	err := db.WithContext(r.Context()).
		Table("workspace_member").
		Select("role").
		Where("workspace_id = ? AND user_id = ? AND status = 1", workspaceID, userID).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return "", false, err
	}
	role := strings.TrimSpace(strings.ToLower(row.Role))
	if role == "" {
		return "", false, nil
	}
	return role, true, nil
}

func resolveCurrentStorageSettingID(ctx context.Context, db *gorm.DB, rdb redis.Cmdable, userID, workspaceID, requestedSettingID string) string {
	//判断是否存在这个存储id
	if settingID := validateStorageSettingID(ctx, db, workspaceID, requestedSettingID); settingID != "" {
		return settingID
	}
	if rdb == nil {
		return ""
	}
	//获取存在redis里面的存储id
	key := CurrentStorageSettingCacheKey(userID, workspaceID)
	cached, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return ""
	}
	//判断存储id是否有效
	if settingID := validateStorageSettingID(ctx, db, workspaceID, cached); settingID != "" {
		return settingID
	}
	_ = rdb.Del(ctx, key).Err()
	return ""
}

func validateStorageSettingID(ctx context.Context, db *gorm.DB, workspaceID, settingID string) string {
	settingID = strings.TrimSpace(settingID)
	workspaceID = strings.TrimSpace(workspaceID)
	if settingID == "" || workspaceID == "" || db == nil {
		return ""
	}

	var count int64
	if err := db.WithContext(ctx).Table("storage_settings").
		Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
		Count(&count).Error; err != nil || count == 0 {
		return ""
	}
	return settingID
}
