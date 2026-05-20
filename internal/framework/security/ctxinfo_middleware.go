package security

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// GinCtxInfoMiddleware 组装请求级 workspace/role/storage 上下文。
//
// 这层解决的是“用户已经知道了，但业务还不知道当前请求落在哪个空间、走哪个存储配置”。
// 处理完成后，后续业务只要读 context，就能拿到：
// - UserID
// - WorkspaceID
// - WorkspaceRole
// - CurrentStorageSettingID
func GinCtxInfoMiddleware(db *gorm.DB, rdb redis.Cmdable) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := GetCtxInfo(c.Request.Context())
		if !ok || strings.TrimSpace(principal.UserID) == "" || db == nil {
			c.Next()
			return
		}

		workspaceID := strings.TrimSpace(c.GetHeader("X-Workspace-Id"))
		candidates := collectWorkspaceCandidates(c.Request, db, principal.UserID, workspaceID, principal.WorkspaceID)
		if len(candidates) == 0 {
			c.Next()
			return
		}
		foundWorkspaceID := ""
		foundRole := ""
		// 候选 workspace 的优先顺序在 collectWorkspaceCandidates 里统一维护。
		// 这里逐个校验成员关系，谁合法就用谁，避免前端伪造 header 越权。
		for _, candidate := range candidates {
			role, found, err := resolveWorkspaceRole(c.Request, db, principal.UserID, candidate)
			if err != nil || !found {
				continue
			}
			foundWorkspaceID = candidate
			foundRole = role
			break
		}
		if foundWorkspaceID == "" {
			c.Next()
			return
		}
		principal.WorkspaceID = foundWorkspaceID
		principal.WorkspaceRole = foundRole
		// workspace 确定后，顺手把当前请求应使用的存储配置也解析好。
		principal.CurrentStorageSettingID = resolveCurrentStorageSettingID(
			c.Request.Context(),
			db,
			rdb,
			principal.UserID,
			foundWorkspaceID,
			c.GetHeader("X-Storage-Setting-Id"),
		)
		c.Request = c.Request.WithContext(PutCtxInfo(c.Request.Context(), principal))
		c.Next()
	}
}
