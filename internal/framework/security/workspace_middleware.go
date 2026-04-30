package security

import (
	"net/http"
	"slices"
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
