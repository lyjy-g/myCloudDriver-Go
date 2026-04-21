package model

import "time"

// Platform 是 service 层存储平台 DTO。
type Platform struct {
	Identifier  string
	Name        string
	Enabled     bool
	Description *string
}

// Setting 是 service 层存储配置 DTO。
type Setting struct {
	ID         string
	Identifier string
	Active     bool
	ConfigJSON *string
	UpdatedAt  *time.Time
}

// CreateSettingInput 是创建存储配置入参。
type CreateSettingInput struct {
	Identifier string
	ConfigJSON string
}

// UpdateSettingInput 是更新存储配置入参。
type UpdateSettingInput struct {
	ConfigJSON string
}
