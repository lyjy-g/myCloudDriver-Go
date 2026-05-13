package model

import "time"

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
	WorkspaceID   string        `json:"workspaceId"`
	WorkspaceName string        `json:"workspaceName,omitempty"`
	SettingID     string        `json:"storageSettingId,omitempty"`
	SettingName   string        `json:"storageSettingName,omitempty"`
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
