package model

import "time"

// 这组 DTO 用于 Gin API 层，避免 service 继续依赖 OpenAPI 生成代码。

type LoginCmd struct {
	IsRemember *bool  `json:"isRemember,omitempty"`
	Password   string `json:"password"`
	Username   string `json:"username"`
}

type PasswordEditRequest struct {
	ConfirmPassword string `json:"confirmPassword"`
	NewPassword     string `json:"newPassword"`
	OldPassword     string `json:"oldPassword"`
}

type PasswordForgetEditRequest struct {
	Code            string `json:"code"`
	ConfirmPassword string `json:"confirmPassword"`
	Mail            string `json:"mail"`
	NewPassword     string `json:"newPassword"`
}

type SysUserResponse struct {
	Avatar      *string    `json:"avatar,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	Email       *string    `json:"email,omitempty"`
	Id          *string    `json:"id,omitempty"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	Nickname    *string    `json:"nickname,omitempty"`
	Status      *int32     `json:"status,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
	Username    *string    `json:"username,omitempty"`
}

type SysUserTransferSetting struct {
	ChunkSize                  *int64     `json:"chunkSize,omitempty"`
	ConcurrentDownloadQuantity *int32     `json:"concurrentDownloadQuantity,omitempty"`
	ConcurrentUploadQuantity   *int32     `json:"concurrentUploadQuantity,omitempty"`
	CreatedAt                  *time.Time `json:"createdAt,omitempty"`
	DownloadLocation           *string    `json:"downloadLocation,omitempty"`
	DownloadSpeedLimit         *int32     `json:"downloadSpeedLimit,omitempty"`
	Id                         *int64     `json:"id,omitempty"`
	IsDefaultDownloadLocation  *int32     `json:"isDefaultDownloadLocation,omitempty"`
	UpdatedAt                  *time.Time `json:"updatedAt,omitempty"`
	UserId                     *string    `json:"userId,omitempty"`
}

type UserEditInfoRequest struct {
	Nickname string `json:"nickname"`
}

type UserRegisterRequest struct {
	Avatar          *string `json:"avatar,omitempty"`
	ConfirmPassword string  `json:"confirmPassword"`
	Email           string  `json:"email"`
	Nickname        string  `json:"nickname"`
	Password        string  `json:"password"`
	Username        string  `json:"username"`
}

type UserTransferSettingEditRequest struct {
	ChunkSize                  int64  `json:"chunkSize"`
	ConcurrentDownloadQuantity int32  `json:"concurrentDownloadQuantity"`
	ConcurrentUploadQuantity   int32  `json:"concurrentUploadQuantity"`
	DownloadLocation           string `json:"downloadLocation"`
	DownloadSpeedLimit         int32  `json:"downloadSpeedLimit"`
	IsDefaultDownloadLocation  int32  `json:"isDefaultDownloadLocation"`
}

type CreateOrgWorkspaceRequest struct {
	Name string `json:"name"`
}

type AddWorkspaceMemberRequest struct {
	Role   string `json:"role"`
	UserId string `json:"userId"`
}
