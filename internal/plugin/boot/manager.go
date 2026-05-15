package boot

import (
	"context"
	"fmt"
	"sync"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/plugin"

	"golang.org/x/sync/singleflight"
)

// cachedStore 表示缓存中的已构建存储实例。
type cachedStore struct {
	storeInfo  plugin.StoreInfo
	storePower plugin.StorePower
}

// Manager 负责存储实例的懒加载、缓存与失效。
// 该结构位于 boot 包，集中承载“插件启动控制”职责。
type Manager struct {
	registry *Registry

	mu sync.RWMutex

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
func (m *Manager) Resolve(ctx context.Context, cfg plugin.ResolvedStorageConfig) (plugin.StorePower, error) {
	m.mu.RLock()
	//如果cache里有了，就直接返回
	if cs, ok := m.cache[cfg.SettingID]; ok &&
		cs.storeInfo != nil &&
		cs.storeInfo.PlatformIdentifier() == cfg.PlatformIdentifier {
		m.mu.RUnlock()
		return cs.storePower, nil
	}
	m.mu.RUnlock()

	v, err, _ := m.sf.Do(cfg.SettingID, func() (any, error) {
		m.mu.RLock()
		if cs, ok := m.cache[cfg.SettingID]; ok &&
			cs.storeInfo != nil &&
			cs.storeInfo.PlatformIdentifier() == cfg.PlatformIdentifier {
			m.mu.RUnlock()
			return cs.storePower, nil
		}
		m.mu.RUnlock()

		info, ok := m.registry.Get(cfg.PlatformIdentifier)
		if !ok {
			return nil, fmt.Errorf("%w: %s", code.ErrStoreInfoNotFound, cfg.PlatformIdentifier)
		}

		if err := info.ValidateConfig(ctx, cfg.ConfigData); err != nil {
			return nil, fmt.Errorf("validate storage config failed: %w", err)
		}

		store, err := info.Build(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("build store failed: %w", err)
		}

		m.mu.Lock()
		m.cache[cfg.SettingID] = cachedStore{
			storeInfo:  info,
			storePower: store,
		}
		m.mu.Unlock()

		return store, nil
	})
	if err != nil {
		return nil, err
	}

	return v.(plugin.StorePower), nil
}

// Invalidate 使指定配置对应的缓存实例失效。
func (m *Manager) Invalidate(settingID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, settingID)
}
