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
			if workspaceID == "" || workspaceID == principal.WorkspaceID {
				next.ServeHTTP(w, r)
				return
			}

			var count int64
			err := db.WithContext(r.Context()).
				Table("workspace_member").
				Where("workspace_id = ? AND user_id = ? AND status = 1", workspaceID, principal.UserID).
				Count(&count).Error
			if err != nil || count == 0 {
				next.ServeHTTP(w, r)
				return
			}

			principal.WorkspaceID = workspaceID
			next.ServeHTTP(w, r.WithContext(PutCtxInfo(r.Context(), principal)))
		})
	}
}
