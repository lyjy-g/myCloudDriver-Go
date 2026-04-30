package security

import (
	"net/http"
	"strings"

	"gorm.io/gorm"
)

// WorkspaceScopeMiddleware 允许前端在请求头显式传 workspace，并在后端做成员校验后覆盖上下文。
//
// 设计要点：
// 1. 只在“已登录用户”场景生效，未登录请求直接透传；
// 2. workspace 必须是用户成员关系，防止通过伪造 header 越权；
// 3. 校验失败时沿用 JWT 内原 workspace，保证兼容性与可用性。
func WorkspaceScopeMiddleware(db *gorm.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := GetCtxInfo(r.Context())
			if !ok || strings.TrimSpace(principal.UserID) == "" || db == nil {
				next.ServeHTTP(w, r)
				return
			}

			workspaceID := strings.TrimSpace(r.Header.Get("X-Workspace-Id"))
			targetWorkspaceID := principal.WorkspaceID
			if workspaceID != "" {
				targetWorkspaceID = workspaceID
			}
			if strings.TrimSpace(targetWorkspaceID) == "" {
				next.ServeHTTP(w, r)
				return
			}

			role, found, err := resolveWorkspaceRole(r, db, principal.UserID, targetWorkspaceID)
			if err != nil || !found {
				next.ServeHTTP(w, r)
				return
			}
			principal.WorkspaceID = targetWorkspaceID
			principal.WorkspaceRole = role
			next.ServeHTTP(w, r.WithContext(PutCtxInfo(r.Context(), principal)))
		})
	}
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
