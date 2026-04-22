package plugin

import (
	dbmodel "myclouddrive-go/internal/storage/model/model"
	"context"
)
context"

type StorageSettingChangeEvent struct {
	SettingID string
}

type StorageSettingSource interface {
	GetByID(ctx context.Context, settingID string) (StorageSetting, error)
	Watch(ctx context.Context) (<-chan StorageSettingChangeEvent, error)
}
