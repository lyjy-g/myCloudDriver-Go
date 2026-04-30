package service

import "time"

// FileShare 对应分享主表。
type FileShare struct {
	ID               string     `gorm:"column:id;primaryKey"`
	UserID           string     `gorm:"column:user_id"`
	ShareName        string     `gorm:"column:share_name"`
	ShareCode        *string    `gorm:"column:share_code"`
	ExpireTime       *time.Time `gorm:"column:expire_time"`
	Scope            string     `gorm:"column:scope"`
	ViewCount        int        `gorm:"column:view_count"`
	MaxViewCount     *int       `gorm:"column:max_view_count"`
	DownloadCount    int        `gorm:"column:download_count"`
	MaxDownloadCount *int       `gorm:"column:max_download_count"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (FileShare) TableName() string { return "share_info" }

// FileShareItem 对应分享-文件映射表。
type FileShareItem struct {
	ShareID   string    `gorm:"column:share_id;primaryKey"`
	FileID    string    `gorm:"column:file_id;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (FileShareItem) TableName() string { return "share_items" }

// FileShareAccessRecord 对应分享访问记录。
type FileShareAccessRecord struct {
	ID            int64     `gorm:"column:id;primaryKey"`
	ShareID       string    `gorm:"column:share_id"`
	AccessIP      *string   `gorm:"column:access_ip"`
	AccessAddress *string   `gorm:"column:access_address"`
	Browser       *string   `gorm:"column:browser"`
	OS            *string   `gorm:"column:os"`
	AccessTime    time.Time `gorm:"column:access_time"`
}

func (FileShareAccessRecord) TableName() string { return "share_access_record" }

// FileInfo 映射文件表（只读取分享所需字段）。
type FileInfo struct {
	ID        string  `gorm:"column:id;primaryKey"`
	Name      string  `gorm:"column:display_name"`
	Size      int64   `gorm:"column:size"`
	IsDir     bool    `gorm:"column:is_dir"`
	ParentID  *string `gorm:"column:parent_id"`
	ObjectKey *string `gorm:"column:object_key"`
	Deleted   *bool   `gorm:"column:is_deleted"`
}

func (FileInfo) TableName() string { return "file_info" }

// ShareFileVO 表示分享文件视图。
type ShareFileVO struct {
	FileID      string `json:"fileId"`
	FileName    string `json:"fileName"`
	FileSize    int64  `json:"fileSize"`
	Directory   bool   `json:"directory"`
	DownloadURL string `json:"downloadUrl,omitempty"`
}

// ShareVO 表示分享详情。
type ShareVO struct {
	ShareID       string        `json:"shareId"`
	ShareName     string        `json:"shareName"`
	ShareCode     string        `json:"shareCode,omitempty"`
	AllowDownload bool          `json:"allowDownload"`
	ExpireTime    *time.Time    `json:"expireTime,omitempty"`
	ViewCount     int           `json:"viewCount"`
	DownloadCount int           `json:"downloadCount"`
	Status        int           `json:"status"`
	FileIDs       []string      `json:"fileIds,omitempty"`
	Files         []ShareFileVO `json:"files,omitempty"`
	AccessPath    string        `json:"accessPath,omitempty"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

// CreateShareReq 创建分享请求。
type CreateShareReq struct {
	ShareName     string   `json:"shareName"`
	FileIDs       []string `json:"fileIds"`
	ShareCode     string   `json:"shareCode"`
	ExpireSeconds *int64   `json:"expireSeconds"`
	AllowDownload *bool    `json:"allowDownload"`
}

// UpdateShareReq 修改分享请求。
type UpdateShareReq struct {
	ShareName     string   `json:"shareName"`
	FileIDs       []string `json:"fileIds"`
	ShareCode     string   `json:"shareCode"`
	ExpireSeconds *int64   `json:"expireSeconds"`
	AllowDownload *bool    `json:"allowDownload"`
}

// VerifyShareCodeReq 校验提取码请求。
type VerifyShareCodeReq struct {
	ShareID   string `json:"shareId"`
	ShareCode string `json:"shareCode"`
}
