package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	pluginsvc "myclouddrive-go/internal/plugin/service"
	storageModel "myclouddrive-go/internal/storage/model"
	dbmodel "myclouddrive-go/internal/storage/model/dbmodel"
)

const platformCacheKey = "storage:platforms:active"

// StorageService 是 storage 业务的唯一实现。
// 按约定：单实现场景不再抽接口。
type StorageService struct {
	db       *gorm.DB
	rdb      redis.Cmdable
	cacheTTL time.Duration
	runtime  *pluginsvc.Runtime
}

func NewService(db *gorm.DB, rdb redis.Cmdable, cacheTTL time.Duration, runtime *pluginsvc.Runtime) *StorageService {
	if runtime == nil {
		runtime = newStoreRuntime(db)
	}
	return &StorageService{
		db:       db,
		rdb:      rdb,
		cacheTTL: cacheTTL,
		runtime:  runtime,
	}
}

func newStoreRuntime(db *gorm.DB) *pluginsvc.Runtime {
	pluginsvc.Init(db)
	return pluginsvc.GetRuntime(db)
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

func (s *StorageService) ListActivePlatforms(ctx context.Context) ([]storageModel.Platform, error) {
	if s.rdb != nil {
		if cached, err := s.rdb.Get(ctx, platformCacheKey).Result(); err == nil {
			var items []storageModel.Platform
			if unmarshalErr := json.Unmarshal([]byte(cached), &items); unmarshalErr == nil {
				return items, nil
			}
		}
	}

	items, err := s.ListStoragePlatforms(ctx)
	if err != nil {
		return nil, err
	}
	if s.rdb != nil {
		payload, _ := json.Marshal(items)
		_ = s.rdb.Set(ctx, platformCacheKey, payload, s.cacheTTL).Err()
	}
	return items, nil
}

func (s *StorageService) ListStoragePlatforms(ctx context.Context) ([]storageModel.Platform, error) {
	var rows []dbmodel.StoragePlatform
	if err := s.db.WithContext(ctx).Order("id asc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list platforms: %w", err)
	}

	result := make([]storageModel.Platform, 0, len(rows))
	for _, row := range rows {
		item := storageModel.Platform{
			Identifier: row.Identifier,
			Name:       row.Name,
			Enabled:    row.IsDefault == 1,
		}
		if strings.TrimSpace(row.Desc) != "" {
			item.Description = &row.Desc
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *StorageService) GetStoragePlatformByIdentifier(ctx context.Context, identifier string) (*storageModel.Platform, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, code.New(code.BadRequest, "identifier is required")
	}

	var row dbmodel.StoragePlatform
	if err := s.db.WithContext(ctx).Where("identifier = ?", identifier).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, code.New(code.NotFound, "platform not found")
		}
		return nil, fmt.Errorf("get platform: %w", err)
	}

	item := &storageModel.Platform{
		Identifier: row.Identifier,
		Name:       row.Name,
		Enabled:    row.IsDefault == 1,
	}
	if strings.TrimSpace(row.Desc) != "" {
		item.Description = &row.Desc
	}
	return item, nil
}

func (s *StorageService) ListStorageSettings(ctx context.Context) ([]storageModel.Setting, error) {
	workspaceID := currentWorkspaceID(ctx)
	var rows []dbmodel.StorageSetting
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND deleted = ?", workspaceID, false).
		Order("updated_at desc").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}

	result := make([]storageModel.Setting, 0, len(rows))
	for _, row := range rows {
		cfgJSON := row.ConfigData
		updated := row.UpdatedAt
		result = append(result, storageModel.Setting{
			ID:         row.ID,
			Identifier: row.PlatformIdentifier,
			Active:     row.Enabled,
			ConfigJSON: &cfgJSON,
			UpdatedAt:  &updated,
		})
	}
	return result, nil
}

func (s *StorageService) CreateStorageSetting(ctx context.Context, req storageModel.CreateSettingInput) (*storageModel.Setting, error) {
	workspaceID := currentWorkspaceID(ctx)
	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		return nil, code.New(code.BadRequest, "identifier is required")
	}
	cfgJSON, err := normalizeAndValidateJSON(req.ConfigJSON)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	row := &dbmodel.StorageSetting{
		ID:                 fmt.Sprintf("stg_%d", now.UnixNano()),
		WorkspaceID:        workspaceID,
		PlatformIdentifier: identifier,
		ConfigData:         cfgJSON,
		Enabled:            false,
		CreatedAt:          now,
		UpdatedAt:          now,
		Deleted:            false,
	}

	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, fmt.Errorf("create setting: %w", err)
	}
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, platformCacheKey).Err()
	}

	cfg := row.ConfigData
	updated := row.UpdatedAt
	return &storageModel.Setting{
		ID:         row.ID,
		Identifier: row.PlatformIdentifier,
		Active:     row.Enabled,
		ConfigJSON: &cfg,
		UpdatedAt:  &updated,
	}, nil
}

func (s *StorageService) UpdateStorageSetting(ctx context.Context, settingID string, req storageModel.UpdateSettingInput) (*storageModel.Setting, error) {
	workspaceID := currentWorkspaceID(ctx)
	cfgJSON, err := normalizeAndValidateJSON(req.ConfigJSON)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"config_data": cfgJSON,
		"updated_at":  time.Now(),
	}
	result := s.db.WithContext(ctx).
		Model(&dbmodel.StorageSetting{}).
		Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
		Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update setting: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, code.New(code.NotFound, "setting not found")
	}
	s.runtime.Invalidate(settingID)

	var row dbmodel.StorageSetting
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
		First(&row).Error; err != nil {
		return nil, fmt.Errorf("query updated setting: %w", err)
	}

	cfg := row.ConfigData
	ts := row.UpdatedAt
	return &storageModel.Setting{
		ID:         row.ID,
		Identifier: row.PlatformIdentifier,
		Active:     row.Enabled,
		ConfigJSON: &cfg,
		UpdatedAt:  &ts,
	}, nil
}

func (s *StorageService) DeleteStorageSetting(ctx context.Context, settingID string) error {
	workspaceID := currentWorkspaceID(ctx)
	result := s.db.WithContext(ctx).
		Model(&dbmodel.StorageSetting{}).
		Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
		Updates(map[string]any{"deleted": true, "updated_at": time.Now()})
	if result.Error != nil {
		return fmt.Errorf("delete setting: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return code.New(code.NotFound, "setting not found")
	}
	s.runtime.Invalidate(settingID)
	return nil
}

func (s *StorageService) ActivateStorageSetting(ctx context.Context, settingID string) (*storageModel.Setting, error) {
	workspaceID := currentWorkspaceID(ctx)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&dbmodel.StorageSetting{}).
			Where("workspace_id = ? AND deleted = ?", workspaceID, false).
			Update("enabled", false).Error; err != nil {
			return err
		}

		res := tx.Model(&dbmodel.StorageSetting{}).
			Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
			Updates(map[string]any{"enabled": true, "updated_at": time.Now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return code.New(code.NotFound, "setting not found")
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("activate setting: %w", err)
	}
	s.runtime.Invalidate(settingID)

	if s.rdb != nil {
		_ = s.rdb.Del(ctx, platformCacheKey).Err()
	}

	var row dbmodel.StorageSetting
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
		First(&row).Error; err != nil {
		return nil, fmt.Errorf("query active setting: %w", err)
	}

	cfg := row.ConfigData
	ts := row.UpdatedAt
	return &storageModel.Setting{
		ID:         row.ID,
		Identifier: row.PlatformIdentifier,
		Active:     row.Enabled,
		ConfigJSON: &cfg,
		UpdatedAt:  &ts,
	}, nil
}

// DisableStorageSetting 禁用指定存储配置。
func (s *StorageService) DisableStorageSetting(ctx context.Context, settingID string) (*storageModel.Setting, error) {
	workspaceID := currentWorkspaceID(ctx)
	res := s.db.WithContext(ctx).
		Model(&dbmodel.StorageSetting{}).
		Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
		Updates(map[string]any{"enabled": false, "updated_at": time.Now()})
	if res.Error != nil {
		return nil, fmt.Errorf("disable setting: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, code.New(code.NotFound, "setting not found")
	}
	s.runtime.Invalidate(settingID)
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, platformCacheKey).Err()
	}

	var row dbmodel.StorageSetting
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
		First(&row).Error; err != nil {
		return nil, fmt.Errorf("query disabled setting: %w", err)
	}
	cfg := row.ConfigData
	ts := row.UpdatedAt
	return &storageModel.Setting{
		ID:         row.ID,
		Identifier: row.PlatformIdentifier,
		Active:     row.Enabled,
		ConfigJSON: &cfg,
		UpdatedAt:  &ts,
	}, nil
}

// Put 将对象写入当前激活存储。
func (s *StorageService) Put(ctx context.Context, in ObjectPutInput) (ObjectInfo, error) {
	info, err := s.runtime.Put(ctx, in.Key, in.Reader, pluginsvc.PutOptions{
		ContentType:   in.ContentType,
		ContentLength: in.ContentLength,
		Metadata:      in.Metadata,
	})
	if err != nil {
		return ObjectInfo{}, err
	}
	return toObjectInfo(info), nil
}

// Get 从当前激活存储读取对象。
func (s *StorageService) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	rc, info, err := s.runtime.Get(ctx, key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return rc, toObjectInfo(info), nil
}

// Delete 删除当前激活存储对象。
func (s *StorageService) Delete(ctx context.Context, key string) error {
	return s.runtime.Delete(ctx, key)
}

// Stat 查询当前激活存储对象元数据。
func (s *StorageService) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	info, err := s.runtime.Stat(ctx, key)
	if err != nil {
		return ObjectInfo{}, err
	}
	return toObjectInfo(info), nil
}

// PresignDownloadURL 生成下载预签名 URL。
func (s *StorageService) PresignDownloadURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return s.runtime.PresignGet(ctx, key, expire)
}

// PresignUploadURL 生成上传预签名 URL。
func (s *StorageService) PresignUploadURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return s.runtime.PresignPut(ctx, key, expire)
}

func toObjectInfo(info pluginsvc.ObjectInfo) ObjectInfo {
	return ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		LastModified: info.LastModified,
	}
}

func normalizeJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return trimmed
	}
	normalized, err := json.Marshal(v)
	if err != nil {
		return trimmed
	}
	return string(normalized)
}

func normalizeAndValidateJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "{}", nil
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return "", code.New(code.BadRequest, "config_json must be valid json")
	}
	normalized, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("normalize config_json failed: %w", err)
	}
	return string(normalized), nil
}

// currentWorkspaceID 示例里先固定，真实项目应改为鉴权中间件或请求头注入。
func currentWorkspaceID(ctx context.Context) string {
	if p, ok := security.PrincipalFromContext(ctx); ok && strings.TrimSpace(p.WorkspaceID) != "" {
		return p.WorkspaceID
	}
	return "ws_01jrvgs943q0f43h0aa5mjde0y_personal"
}
