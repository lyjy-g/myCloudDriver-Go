package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	storageModel "myclouddrive-go/internal/storage/model"
	dbmodel "myclouddrive-go/internal/storage/model/model"
)

const platformCacheKey = "storage:platforms:active"

// StorageService 是 storage 业务的唯一实现。
// 按约定：单实现场景不再抽接口。
type StorageService struct {
	db       *gorm.DB
	rdb      redis.Cmdable
	cacheTTL time.Duration
}

func NewService(db *gorm.DB, rdb redis.Cmdable, cacheTTL time.Duration) *StorageService {
	return &StorageService{db: db, rdb: rdb, cacheTTL: cacheTTL}
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
	var row dbmodel.StoragePlatform
	if err := s.db.WithContext(ctx).Where("identifier = ?", identifier).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("platform not found")
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
	userID := currentUserID(ctx)
	var rows []dbmodel.StorageSetting
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND deleted = ?", userID, false).
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
	userID := currentUserID(ctx)
	now := time.Now()
	row := &dbmodel.StorageSetting{
		ID:                 fmt.Sprintf("stg_%d", now.UnixNano()),
		UserID:             userID,
		PlatformIdentifier: req.Identifier,
		ConfigData:         normalizeJSON(req.ConfigJSON),
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
	userID := currentUserID(ctx)
	updates := map[string]any{
		"config_data": normalizeJSON(req.ConfigJSON),
		"updated_at":  time.Now(),
	}
	result := s.db.WithContext(ctx).
		Model(&dbmodel.StorageSetting{}).
		Where("id = ? AND user_id = ? AND deleted = ?", settingID, userID, false).
		Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update setting: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.New("setting not found")
	}

	var row dbmodel.StorageSetting
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted = ?", settingID, userID, false).
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
	userID := currentUserID(ctx)
	result := s.db.WithContext(ctx).
		Model(&dbmodel.StorageSetting{}).
		Where("id = ? AND user_id = ? AND deleted = ?", settingID, userID, false).
		Updates(map[string]any{"deleted": true, "updated_at": time.Now()})
	if result.Error != nil {
		return fmt.Errorf("delete setting: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("setting not found")
	}
	return nil
}

func (s *StorageService) ActivateStorageSetting(ctx context.Context, settingID string) (*storageModel.Setting, error) {
	userID := currentUserID(ctx)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&dbmodel.StorageSetting{}).
			Where("user_id = ? AND deleted = ?", userID, false).
			Update("enabled", false).Error; err != nil {
			return err
		}

		res := tx.Model(&dbmodel.StorageSetting{}).
			Where("id = ? AND user_id = ? AND deleted = ?", settingID, userID, false).
			Updates(map[string]any{"enabled": true, "updated_at": time.Now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("setting not found")
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("activate setting: %w", err)
	}

	if s.rdb != nil {
		_ = s.rdb.Del(ctx, platformCacheKey).Err()
	}

	var row dbmodel.StorageSetting
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND deleted = ?", settingID, userID, false).
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

// currentUserID 示例里先固定，真实项目应改为鉴权中间件注入。
func currentUserID(_ context.Context) string {
	return "demo-user"
}
