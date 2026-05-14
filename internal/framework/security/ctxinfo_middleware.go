package security

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// CtxInfoMiddleware 允许前端在请求头显式传 workspace，并在后端做成员校验后覆盖上下文。
//
// 设计要点：
// 1. 只在“已登录用户”场景生效，未登录请求直接透传；
// 2. workspace 必须是用户成员关系，防止通过伪造 header 越权；
// 3. 校验失败时沿用 JWT 内原 workspace，保证兼容性与可用性；
// 4. workspace 确认后，顺手把当前请求应使用的存储配置一并注入上下文。
func CtxInfoMiddleware(db *gorm.DB, rdb redis.Cmdable) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := GetCtxInfo(r.Context())
			if !ok || strings.TrimSpace(principal.UserID) == "" || db == nil {
				next.ServeHTTP(w, r)
				return
			}

			workspaceID := strings.TrimSpace(r.Header.Get("X-Workspace-Id"))
			candidates := collectWorkspaceCandidates(r, db, principal.UserID, workspaceID, principal.WorkspaceID)
			if len(candidates) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			foundWorkspaceID := ""
			foundRole := ""
			for _, candidate := range candidates {
				role, found, err := resolveWorkspaceRole(r, db, principal.UserID, candidate)
				if err != nil || !found {
					continue
				}
				foundWorkspaceID = candidate
				foundRole = role
				break
			}
			if foundWorkspaceID == "" {
				next.ServeHTTP(w, r)
				return
			}
			principal.WorkspaceID = foundWorkspaceID
			principal.WorkspaceRole = foundRole
			//设置存储id
			principal.CurrentStorageSettingID = resolveCurrentStorageSettingID(r.Context(), db, rdb, principal.UserID, foundWorkspaceID, r.Header.Get("X-Storage-Setting-Id"))
			next.ServeHTTP(w, r.WithContext(PutCtxInfo(r.Context(), principal)))
		})
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
