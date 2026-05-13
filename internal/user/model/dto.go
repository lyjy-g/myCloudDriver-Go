package model

import "time"

// LoginResult 登录成功返回体（不在 OpenAPI components 中，归档到 dto.go）。
type LoginResult struct {
	Token       string  `json:"token"`
	UserID      string  `json:"userId"`
	Username    string  `json:"username"`
	Nickname    string  `json:"nickname"`
	Email       string  `json:"email"`
	Avatar      *string `json:"avatar,omitempty"`
	WorkspaceID string  `json:"workspaceId"`
}

// WorkspaceItem 工作空间列表项。
type WorkspaceItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	WorkspaceType string `json:"workspaceType"`
	Role          string `json:"role"`
	IsDefault     bool   `json:"isDefault"`
}

// WorkspaceMemberItem 工作空间成员列表项。
type WorkspaceMemberItem struct {
	UserID   string    `json:"userId"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joinedAt"`
}

// ActiveWorkspaceCmd 兼容前端 active workspace 切换请求。
type ActiveWorkspaceCmd struct {
	WorkspaceID string `json:"workspaceId"`
}
