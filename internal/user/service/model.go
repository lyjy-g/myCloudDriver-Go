package service

import "time"

// SysUser 对应用户表。
type SysUser struct {
	ID                 string     `gorm:"column:id;primaryKey"`
	Username           string     `gorm:"column:username"`
	Password           string     `gorm:"column:password"`
	Email              string     `gorm:"column:email"`
	Nickname           string     `gorm:"column:nickname"`
	Avatar             *string    `gorm:"column:avatar"`
	DefaultWorkspaceID *string    `gorm:"column:default_workspace_id"`
	Status             int        `gorm:"column:status"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
	LastLoginAt        *time.Time `gorm:"column:last_login_at"`
}

func (SysUser) TableName() string { return "sys_user" }

// Workspace 对应工作空间表。
type Workspace struct {
	ID            string    `gorm:"column:id;primaryKey"`
	Name          string    `gorm:"column:name"`
	WorkspaceType string    `gorm:"column:workspace_type"`
	OwnerUserID   string    `gorm:"column:owner_user_id"`
	Status        int8      `gorm:"column:status"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (Workspace) TableName() string { return "workspace" }

// WorkspaceMember 对应工作空间成员表。
type WorkspaceMember struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	WorkspaceID string    `gorm:"column:workspace_id"`
	UserID      string    `gorm:"column:user_id"`
	Role        string    `gorm:"column:role"`
	Status      int8      `gorm:"column:status"`
	JoinedAt    time.Time `gorm:"column:joined_at"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (WorkspaceMember) TableName() string { return "workspace_member" }

// SysUserTransferSetting 对应用户传输设置表。
type SysUserTransferSetting struct {
	ID                         int64     `gorm:"column:id;primaryKey"`
	UserID                     string    `gorm:"column:user_id"`
	DownloadLocation           string    `gorm:"column:download_location"`
	IsDefaultDownloadLocation  int       `gorm:"column:is_default_download_location"`
	DownloadSpeedLimit         int       `gorm:"column:download_speed_limit"`
	ConcurrentUploadQuantity   int       `gorm:"column:concurrent_upload_quantity"`
	ConcurrentDownloadQuantity int       `gorm:"column:concurrent_download_quantity"`
	ChunkSize                  int64     `gorm:"column:chunk_size"`
	CreatedAt                  time.Time `gorm:"column:created_at"`
	UpdatedAt                  time.Time `gorm:"column:updated_at"`
}

func (SysUserTransferSetting) TableName() string { return "sys_user_transfer_setting" }

// LoginLog 对应登录审计表。
type LoginLog struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	UserID       *string   `gorm:"column:user_id"`
	Username     string    `gorm:"column:username"`
	LoginIP      string    `gorm:"column:login_ip"`
	LoginAddress *string   `gorm:"column:login_address"`
	Browser      *string   `gorm:"column:browser"`
	OS           string    `gorm:"column:os"`
	Status       int       `gorm:"column:status"`
	Msg          string    `gorm:"column:msg"`
	LoginTime    time.Time `gorm:"column:login_time"`
}

func (LoginLog) TableName() string { return "sys_login_log" }

// LoginResult 登录响应。
type LoginResult struct {
	Token       string  `json:"token"`
	UserID      string  `json:"userId"`
	Username    string  `json:"username"`
	Nickname    string  `json:"nickname"`
	Email       string  `json:"email"`
	Avatar      *string `json:"avatar,omitempty"`
	WorkspaceID string  `json:"workspaceId"`
}

// UserInfo 用户信息响应。
type UserInfo struct {
	ID                 string  `json:"id"`
	Username           string  `json:"username"`
	Email              string  `json:"email"`
	Nickname           string  `json:"nickname"`
	Avatar             *string `json:"avatar,omitempty"`
	DefaultWorkspaceID string  `json:"defaultWorkspaceId"`
	Status             int     `json:"status"`
}

// TransferSettingInput 编辑传输配置请求。
type TransferSettingInput struct {
	DownloadLocation           string `json:"downloadLocation"`
	IsDefaultDownloadLocation  int    `json:"isDefaultDownloadLocation"`
	DownloadSpeedLimit         int    `json:"downloadSpeedLimit"`
	ConcurrentUploadQuantity   int    `json:"concurrentUploadQuantity"`
	ConcurrentDownloadQuantity int    `json:"concurrentDownloadQuantity"`
	ChunkSize                  int64  `json:"chunkSize"`
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
