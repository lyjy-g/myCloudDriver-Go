package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	pluginsvc "myclouddrive-go/internal/plugin/service"
	dto "myclouddrive-go/internal/storage/model"
	storageModel "myclouddrive-go/internal/storage/model"
	dbmodel "myclouddrive-go/internal/storage/model/dbmodel"
)

const platformCacheKey = "storage:platforms:active"
const defaultLocalStorageRoot = ".data"

// StorageService 是 storage 业务的唯一实现。
// 按约定：单实现场景不再抽接口。
// 1. service 层只负责编排，不直接依赖具体存储 SDK；
// 2. 通过 runManager 把“配置解析 + 插件实例选择”下沉到插件模块；
// 3. 配置变更后做精确缓存失效，保障运行时一致性。
type StorageService struct {
	db         *gorm.DB
	rdb        redis.Cmdable
	cacheTTL   time.Duration
	runManager *pluginsvc.RunManager
}

// NewService 构造存储业务服务。
//
// - 支持外部注入 runManager，方便测试和模块化装配；
// - runManager 为空时兜底获取单例，保证服务可独立运行。
func NewService(db *gorm.DB, rdb redis.Cmdable, cacheTTL time.Duration, runManager *pluginsvc.RunManager) *StorageService {
	if runManager == nil {
		// GetRunManager 内部有 once 控制，避免重复初始化。
		runManager = pluginsvc.GetRunManager(db)
	}
	return &StorageService{
		db:         db,
		rdb:        rdb,
		cacheTTL:   cacheTTL,
		runManager: runManager,
	}
}

// ListStorageSettings 查询当前工作空间配置。
// - workspace_id 是多租户隔离关键字段；
// - deleted 逻辑删除避免直接物理删造成审计缺失。
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
			ID:                 row.ID,
			StorageSettingName: row.StorageSettingName,
			Identifier:         row.PlatformIdentifier,
			Active:             row.Enabled,
			ConfigJSON:         &cfgJSON,
			UpdatedAt:          &updated,
		})
	}
	return result, nil
}

// CreateStorageSetting 创建存储配置（默认未启用）。
// - 新配置默认不启用，避免误切流；
// - 入库前做 JSON 校验与归一化，减少后续运行时失败。
func (s *StorageService) CreateStorageSetting(ctx context.Context, req storageModel.CreateSettingInput) (*storageModel.Setting, error) {
	workspaceID := currentWorkspaceID(ctx)
	identifier := strings.ToLower(strings.TrimSpace(req.Identifier))
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
		StorageSettingName: strings.TrimSpace(req.StorageSettingName),
		WorkspaceID:        workspaceID,
		PlatformIdentifier: identifier,
		ConfigData:         cfgJSON,
		Enabled:            false,
		CreatedAt:          now,
		UpdatedAt:          now,
		Deleted:            false,
	}
	if row.StorageSettingName == "" {
		row.StorageSettingName = fmt.Sprintf("%s-%s", strings.ToLower(identifier), row.ID)
	}
	cfgJSON, err = normalizeStorageConfig(identifier, workspaceID, row.ID, cfgJSON)
	if err != nil {
		return nil, err
	}
	row.ConfigData = cfgJSON

	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, fmt.Errorf("create setting: %w", err)
	}
	if s.rdb != nil {
		// 配置变化后清理相关缓存，避免控制台读到旧值。
		_ = s.rdb.Del(ctx, platformCacheKey).Err()
	}

	cfg := row.ConfigData
	updated := row.UpdatedAt
	return &storageModel.Setting{
		ID:                 row.ID,
		StorageSettingName: row.StorageSettingName,
		Identifier:         row.PlatformIdentifier,
		Active:             row.Enabled,
		ConfigJSON:         &cfg,
		UpdatedAt:          &updated,
	}, nil
}

// UpdateStorageSetting 更新配置并失效插件实例缓存。
func (s *StorageService) UpdateStorageSetting(ctx context.Context, settingID string, req storageModel.UpdateSettingInput) (*storageModel.Setting, error) {
	workspaceID := currentWorkspaceID(ctx)
	var existing dbmodel.StorageSetting
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
		First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, code.New(code.NotFound, "setting not found")
		}
		return nil, fmt.Errorf("query setting before update: %w", err)
	}
	cfgJSON, err := normalizeAndValidateJSON(req.ConfigJSON)
	if err != nil {
		return nil, err
	}
	cfgJSON, err = normalizeStorageConfig(existing.PlatformIdentifier, workspaceID, settingID, cfgJSON)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"config_data": cfgJSON,
		"updated_at":  time.Now(),
	}
	if identifier := strings.ToLower(strings.TrimSpace(existing.PlatformIdentifier)); identifier != "" {
		updates["platform_identifier"] = identifier
	}
	if req.StorageSettingName != nil {
		updates["storage_setting_name"] = strings.TrimSpace(*req.StorageSettingName)
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
	// 配置更新后，旧实例必须失效，下一次访问由 runManager 懒加载重建。
	s.runManager.Invalidate(settingID)

	var row dbmodel.StorageSetting
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND deleted = ?", settingID, workspaceID, false).
		First(&row).Error; err != nil {
		return nil, fmt.Errorf("query updated setting: %w", err)
	}

	cfg := row.ConfigData
	ts := row.UpdatedAt
	return &storageModel.Setting{
		ID:                 row.ID,
		StorageSettingName: row.StorageSettingName,
		Identifier:         row.PlatformIdentifier,
		Active:             row.Enabled,
		ConfigJSON:         &cfg,
		UpdatedAt:          &ts,
	}, nil
}

// DeleteStorageSetting 逻辑删除配置并失效实例缓存。
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
	s.runManager.Invalidate(settingID)
	return nil
}

// ActivateStorageSetting 激活指定配置，保证同一 workspace 仅有一个启用项。
//
// 面试可讲：
// - 使用事务实现“全量禁用 + 单条启用”的原子切换；
// - 数据提交后再失效插件缓存，避免状态错序。
func (s *StorageService) ActivateStorageSetting(ctx context.Context, settingID string) (*storageModel.Setting, error) {
	workspaceID := currentWorkspaceID(ctx)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 步骤1：先将当前空间所有配置设为禁用。
		if err := tx.Model(&dbmodel.StorageSetting{}).
			Where("workspace_id = ? AND deleted = ?", workspaceID, false).
			Update("enabled", false).Error; err != nil {
			return err
		}

		// 步骤2：再启用目标配置。
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
	s.runManager.Invalidate(settingID)

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
		ID:                 row.ID,
		StorageSettingName: row.StorageSettingName,
		Identifier:         row.PlatformIdentifier,
		Active:             row.Enabled,
		ConfigJSON:         &cfg,
		UpdatedAt:          &ts,
	}, nil
}

// DisableStorageSetting 禁用指定存储配置。
//
// 面试可讲：
// - 禁用后立即失效实例缓存，避免旧配置继续承接流量；
// - 同步清理 Redis 缓存，保证控制台状态一致。
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
	s.runManager.Invalidate(settingID)
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
		ID:                 row.ID,
		StorageSettingName: row.StorageSettingName,
		Identifier:         row.PlatformIdentifier,
		Active:             row.Enabled,
		ConfigJSON:         &cfg,
		UpdatedAt:          &ts,
	}, nil
}

// Put 将对象写入当前激活存储。
//
// 可忽略：
// - 该函数主要做入参映射，不承担复杂业务决策。
func (s *StorageService) Put(ctx context.Context, in dto.ObjectPutInput) (dto.ObjectInfo, error) {
	info, err := s.runManager.Put(ctx, in.Key, in.Reader, pluginsvc.PutOptions{
		ContentType:   in.ContentType,
		ContentLength: in.ContentLength,
		Metadata:      in.Metadata,
	})
	if err != nil {
		return dto.ObjectInfo{}, err
	}
	return toObjectInfo(info), nil
}

// Get 从当前激活存储读取对象。
func (s *StorageService) Get(ctx context.Context, key string) (io.ReadCloser, dto.ObjectInfo, error) {
	rc, info, err := s.runManager.Get(ctx, key)
	if err != nil {
		return nil, dto.ObjectInfo{}, err
	}
	return rc, toObjectInfo(info), nil
}

// Delete 删除当前激活存储对象。
func (s *StorageService) Delete(ctx context.Context, key string) error {
	return s.runManager.Delete(ctx, key)
}

// Stat 查询当前激活存储对象元数据。
func (s *StorageService) Stat(ctx context.Context, key string) (dto.ObjectInfo, error) {
	info, err := s.runManager.Stat(ctx, key)
	if err != nil {
		return dto.ObjectInfo{}, err
	}
	return toObjectInfo(info), nil
}

// PresignDownloadURL 生成下载预签名 URL。
//
// 面试可讲：
// - file 模块只依赖该门面，不感知 local/s3 的签名实现差异。
func (s *StorageService) PresignDownloadURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return s.runManager.PresignGet(ctx, key, expire)
}

// PresignUploadURL 生成上传预签名 URL。
func (s *StorageService) PresignUploadURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return s.runManager.PresignPut(ctx, key, expire)
}

// toObjectInfo 将插件层对象信息映射为 storage DTO。
func toObjectInfo(info pluginsvc.ObjectInfo) dto.ObjectInfo {
	return dto.ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		LastModified: info.LastModified,
	}
}

// normalizeJSON 尝试将 JSON 文本标准化（不严格校验）。
//
// 可忽略：
// - 当前主流程未使用，仅保留工具方法。
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

// normalizeAndValidateJSON 对配置 JSON 做严格校验与归一化。
//
// 面试可讲：
// - 在入库层面做防线，避免脏配置进入运行时导致插件 Build 失败。
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

// normalizeStorageConfig 根据平台规范化配置：
// - local: 强制由后端生成 base_path，前端不允许自定义绝对路径；
// - s3: 强制注入 prefix=workspaceID/settingID，实现与 local 一致的分租户隔离。
func normalizeStorageConfig(identifier, workspaceID, settingID, raw string) (string, error) {
	cfgJSON, err := normalizeAndValidateJSON(raw)
	if err != nil {
		return "", err
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return "", fmt.Errorf("unmarshal config_json failed: %w", err)
	}

	normalizedIdentifier := strings.ToLower(strings.TrimSpace(identifier))
	if normalizedIdentifier == "" {
		if v, ok := cfg["identifier"].(string); ok {
			normalizedIdentifier = strings.ToLower(strings.TrimSpace(v))
		}
	}
	if normalizedIdentifier == "" || normalizedIdentifier == "local" {
		root := strings.TrimSpace(os.Getenv("MYCLOUDDRIVE_LOCAL_ROOT"))
		if root == "" {
			root = defaultLocalStorageRoot
		}
		localPath := filepath.ToSlash(filepath.Join(root, workspaceID, settingID))
		cfg["base_path"] = localPath
	} else if normalizedIdentifier == "s3" {
		isolationPrefix := strings.Trim(filepath.ToSlash(filepath.Join(".data", workspaceID, settingID)), "/")
		existingPrefix, _ := cfg["prefix"].(string)
		existingPrefix = strings.Trim(existingPrefix, "/")
		// 兼容前端可选 namespace/prefix：最终都挂到 workspace/setting 下，避免跨空间串数据。
		if existingPrefix == "" {
			cfg["prefix"] = isolationPrefix
		} else {
			cfg["prefix"] = isolationPrefix + "/" + existingPrefix
		}
	}

	normalized, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal normalized config_json failed: %w", err)
	}
	return string(normalized), nil
}

// currentWorkspaceID 从上下文提取当前工作空间。
//
// 面试可讲：
// - workspace 是多租户隔离的关键维度；
// - 真实生产建议强依赖鉴权注入，而不是兜底默认值。
func currentWorkspaceID(ctx context.Context) string {
	if p, ok := security.GetCtxInfo(ctx); ok && strings.TrimSpace(p.WorkspaceID) != "" {
		return p.WorkspaceID
	}
	return "ws_01jrvgs943q0f43h0aa5mjde0y_personal"
}

// ListActivePlatforms 返回平台列表（带缓存）。
// - 使用 Cache-Aside：先查 Redis，未命中查库并回填；
// - 缓存失败不影响主流程，可用性优先。
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
		// 回填失败不返回错误，避免缓存依赖放大业务故障。
		payload, _ := json.Marshal(items)
		_ = s.rdb.Set(ctx, platformCacheKey, payload, s.cacheTTL).Err()
	}
	return items, nil
}

// ListStoragePlatforms 查询平台元数据（DB 真值）。
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

// GetStoragePlatformByIdentifier 按标识读取单个平台。
func (s *StorageService) GetStoragePlatformByIdentifier(ctx context.Context, identifier string) (*storageModel.Platform, error) {
	//pre
	identifier = strings.TrimSpace(identifier)
	//null
	if identifier == "" {
		return nil, code.New(code.BadRequest, "identifier is required")
	}
	//db
	var row dbmodel.StoragePlatform
	if err := s.db.WithContext(ctx).Where("identifier = ?", identifier).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, code.New(code.NotFound, "platform not found")
		}
		return nil, fmt.Errorf("get platform: %w", err)
	}
	//convert
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
