package model

import (
	"io"
	"time"
)

// Platform 是 service 层存储平台 DTO。
type Platform struct {
	Identifier  string
	Name        string
	Enabled     bool
	Description *string
}

// Setting 是 service 层存储配置 DTO。
type Setting struct {
	ID                 string
	StorageSettingName string
	Identifier         string
	Active             bool
	ConfigJSON         *string
	UpdatedAt          *time.Time
}

// CreateSettingInput 是创建存储配置入参。
type CreateSettingInput struct {
	StorageSettingName string
	Identifier         string
	ConfigJSON         string
}

// UpdateSettingInput 是更新存储配置入参。
type UpdateSettingInput struct {
	StorageSettingName *string
	ConfigJSON         string
}

// ObjectPutInput 是业务层写入对象入参，不暴露插件底层类型。
type ObjectPutInput struct {
	Key           string
	Reader        io.Reader
	ContentType   string
	ContentLength *int64
	Metadata      map[string]string
}

// ObjectInfo 是业务层对象元信息。
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified *time.Time
}
