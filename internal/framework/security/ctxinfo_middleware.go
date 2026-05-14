package security

import (
	"net/http"
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
