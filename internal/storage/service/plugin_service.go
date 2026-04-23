package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"myclouddrive-go/internal/framework/code"
	dbmodel "myclouddrive-go/internal/storage/model/dbmodel"
	modelgen "myclouddrive-go/internal/storage/model/gen"
	"myclouddrive-go/internal/storage/plugin"
	"time"

	"gorm.io/gorm"
)

// ResolveActiveStore 解析当前用户已启用的存储实例。
func (s *StorageService) ResolveActiveStore(ctx context.Context) (plugin.Store, error) {
	row, err := s.getWorkspaceActiveSetting(ctx)
	if err != nil {
		return nil, err
	}
	return s.resolveStoreBySetting(ctx, row)
}

// ResolveStoreBySettingID 按 settingID 解析存储实例。
func (s *StorageService) ResolveStoreBySettingID(ctx context.Context, settingID string) (plugin.Store, error) {
	workspaceID := currentWorkspaceID(ctx)
	q := modelgen.Use(s.db)
	ss := q.StorageSetting

	row, err := q.WithContext(ctx).StorageSetting.
		Where(
			ss.ID.Eq(settingID),
			ss.WorkspaceID.Eq(workspaceID),
			ss.Deleted.Is(false),
		).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %s", code.ErrSettingNotFound, settingID)
		}
		return nil, fmt.Errorf("query storage setting failed: %w", err)
	}
	if !row.Enabled {
		return nil, fmt.Errorf("%w: %s", code.ErrSettingDisabled, settingID)
	}
	return s.resolveStoreBySetting(ctx, *row)
}

// PutObject 使用当前激活存储写入对象。
func (s *StorageService) PutObject(ctx context.Context, key plugin.Key, r io.Reader, opts plugin.PutOptions) (plugin.ObjectInfo, error) {
	store, err := s.ResolveActiveStore(ctx)
	if err != nil {
		return plugin.ObjectInfo{}, err
	}
	return store.Put(ctx, key, r, opts)
}

// GetObject 使用当前激活存储读取对象。
func (s *StorageService) GetObject(ctx context.Context, key plugin.Key) (io.ReadCloser, plugin.ObjectInfo, error) {
	store, err := s.ResolveActiveStore(ctx)
	if err != nil {
		return nil, plugin.ObjectInfo{}, err
	}
	return store.Get(ctx, key)
}

// DeleteObject 使用当前激活存储删除对象。
func (s *StorageService) DeleteObject(ctx context.Context, key plugin.Key) error {
	store, err := s.ResolveActiveStore(ctx)
	if err != nil {
		return err
	}
	return store.Delete(ctx, key)
}

// StatObject 使用当前激活存储查询对象元信息。
func (s *StorageService) StatObject(ctx context.Context, key plugin.Key) (plugin.ObjectInfo, error) {
	store, err := s.ResolveActiveStore(ctx)
	if err != nil {
		return plugin.ObjectInfo{}, err
	}
	return store.Stat(ctx, key)
}

// PresignGet 生成下载预签名 URL。
func (s *StorageService) PresignGet(ctx context.Context, key plugin.Key, expire time.Duration) (string, error) {
	store, err := s.ResolveActiveStore(ctx)
	if err != nil {
		return "", err
	}
	signed, ok := store.(plugin.SignedURLStore)
	if !ok {
		return "", fmt.Errorf("%w: presign get", code.ErrCapabilityNotMatch)
	}
	return signed.PresignGet(ctx, key, expire)
}

// PresignPut 生成上传预签名 URL。
func (s *StorageService) PresignPut(ctx context.Context, key plugin.Key, expire time.Duration) (string, error) {
	store, err := s.ResolveActiveStore(ctx)
	if err != nil {
		return "", err
	}
	signed, ok := store.(plugin.SignedURLStore)
	if !ok {
		return "", fmt.Errorf("%w: presign put", code.ErrCapabilityNotMatch)
	}
	return signed.PresignPut(ctx, key, expire)
}

// InvalidateStoreSetting 主动失效存储实例缓存。
func (s *StorageService) InvalidateStoreSetting(settingID string) {
	s.plugins.Invalidate(settingID)
}

func (s *StorageService) getWorkspaceActiveSetting(ctx context.Context) (dbmodel.StorageSetting, error) {
	workspaceID := currentWorkspaceID(ctx)
	q := modelgen.Use(s.db)
	ss := q.StorageSetting

	row, err := q.WithContext(ctx).StorageSetting.
		Where(
			ss.WorkspaceID.Eq(workspaceID),
			ss.Enabled.Is(true),
			ss.Deleted.Is(false),
		).
		Order(ss.UpdatedAt.Desc()).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dbmodel.StorageSetting{}, fmt.Errorf("%w: active setting for workspace %s", code.ErrSettingNotFound, workspaceID)
		}
		return dbmodel.StorageSetting{}, fmt.Errorf("query active storage setting failed: %w", err)
	}
	return *row, nil
}

func (s *StorageService) resolveStoreBySetting(ctx context.Context, row dbmodel.StorageSetting) (plugin.Store, error) {
	cfg := plugin.ResolvedStorageConfig{
		SettingID:          row.ID,
		WorkspaceID:        row.WorkspaceID,
		PlatformIdentifier: plugin.PlatformIdentifier(row.PlatformIdentifier),
		ConfigData:         []byte(row.ConfigData),
		Version:            plugin.Fingerprint(row),
	}

	store, err := s.plugins.Resolve(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve storage plugin failed: %w", err)
	}
	return store, nil
}
