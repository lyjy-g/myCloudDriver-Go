package boot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"gorm.io/gorm"

	"myclouddrive-go/internal/framework/code"
	dbmodel "myclouddrive-go/internal/storage/model/dbmodel"
	modelgen "myclouddrive-go/internal/storage/model/gen"
	"myclouddrive-go/internal/storage/plugin"

	"golang.org/x/sync/singleflight"
)

// PutOptions 是存储写入选项（业务层可感知字段）。
type PutOptions struct {
	ContentType   string
	ContentLength *int64
	Metadata      map[string]string
}

// ObjectInfo 是对象元信息（屏蔽底层插件细节）。
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified *time.Time
}

// cachedStore 表示缓存中的已构建存储实例。
type cachedStore struct {
	fingerprint string
	store       plugin.Store
}

// Manager 负责存储实例的懒加载、缓存与失效。
// 该结构位于 boot 包，集中承载“插件启动控制”职责。
type Manager struct {
	registry *Registry

	mu    sync.RWMutex
	cache map[string]cachedStore

	sf singleflight.Group
}

// NewManager 创建存储实例管理器。
func NewManager(registry *Registry) *Manager {
	return &Manager{
		registry: registry,
		cache:    make(map[string]cachedStore),
	}
}

// Resolve 根据配置解析并返回可复用的 Store 实例。
func (m *Manager) Resolve(ctx context.Context, cfg plugin.ResolvedStorageConfig) (plugin.Store, error) {
	fp := cfg.Version

	m.mu.RLock()
	if cs, ok := m.cache[cfg.SettingID]; ok && cs.fingerprint == fp {
		m.mu.RUnlock()
		return cs.store, nil
	}
	m.mu.RUnlock()

	v, err, _ := m.sf.Do(cfg.SettingID+":"+fp, func() (any, error) {
		m.mu.RLock()
		if cs, ok := m.cache[cfg.SettingID]; ok && cs.fingerprint == fp {
			m.mu.RUnlock()
			return cs.store, nil
		}
		m.mu.RUnlock()

		d, ok := m.registry.Get(cfg.PlatformIdentifier)
		if !ok {
			return nil, fmt.Errorf("%w: %s", code.ErrDriverNotFound, cfg.PlatformIdentifier)
		}

		if err := d.ValidateConfig(ctx, cfg.ConfigData); err != nil {
			return nil, fmt.Errorf("validate storage config failed: %w", err)
		}

		store, err := d.Build(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("build store failed: %w", err)
		}

		m.mu.Lock()
		m.cache[cfg.SettingID] = cachedStore{
			fingerprint: fp,
			store:       store,
		}
		m.mu.Unlock()

		return store, nil
	})
	if err != nil {
		return nil, err
	}

	return v.(plugin.Store), nil
}

// Invalidate 使指定配置对应的缓存实例失效。
func (m *Manager) Invalidate(settingID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, settingID)
}

// Runtime 负责将“工作空间配置”解析为具体 Store，并提供统一对象操作能力。
type Runtime struct {
	db                  *gorm.DB
	manager             *Manager
	workspaceIDResolver func(context.Context) string
}

// NewRuntime 创建插件运行时服务。
func NewRuntime(db *gorm.DB, manager *Manager, workspaceIDResolver func(context.Context) string) *Runtime {
	return &Runtime{
		db:                  db,
		manager:             manager,
		workspaceIDResolver: workspaceIDResolver,
	}
}

// Put 使用当前工作空间激活的配置写入对象。
func (r *Runtime) Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) (ObjectInfo, error) {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return ObjectInfo{}, err
	}
	info, err := store.Put(ctx, plugin.Key(key), reader, plugin.PutOptions{
		ContentType:   opts.ContentType,
		ContentLength: opts.ContentLength,
		Metadata:      opts.Metadata,
	})
	if err != nil {
		return ObjectInfo{}, err
	}
	return toRuntimeObjectInfo(info), nil
}

// Get 使用当前工作空间激活的配置读取对象。
func (r *Runtime) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	body, info, err := store.Get(ctx, plugin.Key(key))
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return body, toRuntimeObjectInfo(info), nil
}

// Delete 使用当前工作空间激活的配置删除对象。
func (r *Runtime) Delete(ctx context.Context, key string) error {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return err
	}
	return store.Delete(ctx, plugin.Key(key))
}

// Stat 使用当前工作空间激活的配置查询对象元信息。
func (r *Runtime) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return ObjectInfo{}, err
	}
	info, err := store.Stat(ctx, plugin.Key(key))
	if err != nil {
		return ObjectInfo{}, err
	}
	return toRuntimeObjectInfo(info), nil
}

// PresignGet 使用当前工作空间激活配置生成下载预签名地址。
func (r *Runtime) PresignGet(ctx context.Context, key string, expire time.Duration) (string, error) {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return "", err
	}
	signed, ok := store.(plugin.SignedURLStore)
	if !ok {
		return "", fmt.Errorf("%w: presign get", code.ErrCapabilityNotMatch)
	}
	return signed.PresignGet(ctx, plugin.Key(key), expire)
}

// PresignPut 使用当前工作空间激活配置生成上传预签名地址。
func (r *Runtime) PresignPut(ctx context.Context, key string, expire time.Duration) (string, error) {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return "", err
	}
	signed, ok := store.(plugin.SignedURLStore)
	if !ok {
		return "", fmt.Errorf("%w: presign put", code.ErrCapabilityNotMatch)
	}
	return signed.PresignPut(ctx, plugin.Key(key), expire)
}

// Invalidate 主动失效指定配置对应的缓存实例。
func (r *Runtime) Invalidate(settingID string) {
	r.manager.Invalidate(settingID)
}

func (r *Runtime) resolveActiveStore(ctx context.Context) (plugin.Store, error) {
	row, err := r.getWorkspaceActiveSetting(ctx)
	if err != nil {
		return nil, err
	}
	return r.resolveStoreBySetting(ctx, row)
}

func (r *Runtime) getWorkspaceActiveSetting(ctx context.Context) (dbmodel.StorageSetting, error) {
	workspaceID := r.workspaceIDResolver(ctx)
	q := modelgen.Use(r.db)
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

func (r *Runtime) resolveStoreBySetting(ctx context.Context, row dbmodel.StorageSetting) (plugin.Store, error) {
	cfg := plugin.ResolvedStorageConfig{
		SettingID:          row.ID,
		WorkspaceID:        row.WorkspaceID,
		PlatformIdentifier: plugin.PlatformIdentifier(row.PlatformIdentifier),
		ConfigData:         []byte(row.ConfigData),
		Version:            plugin.Fingerprint(row),
	}

	store, err := r.manager.Resolve(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve storage plugin failed: %w", err)
	}
	return store, nil
}

func toRuntimeObjectInfo(info plugin.ObjectInfo) ObjectInfo {
	return ObjectInfo{
		Key:          string(info.Key),
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		LastModified: info.LastModified,
	}
}
