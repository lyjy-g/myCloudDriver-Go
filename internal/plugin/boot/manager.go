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
