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

// Resolve 懒加载的核心: 三级兜底返回 StorePower。
//
//	第一级: 读锁查缓存 → 命中直接返回。
//	第二级: singleflight 确保同一 settingID 并发时只 build 一次，并在 build 前 double-check 缓存。
//	第三级: 注册表取出 StoreInfo → Validate → Build（这是"重"操作，如建 S3 client），结果写回缓存。
//
// 缓存 key 是 settingID，但缓存的校验加了 platformIdentifier，
// 这样即使 settingID 不变但换了平台（如 local→s3），也能识别并重建。
func (m *Manager) Resolve(ctx context.Context, cfg plugin.ResolvedStorageConfig) (plugin.StorePower, error) {
	//第一级: 读锁查缓存 → 命中直接返回。
	m.mu.RLock()
	if cs, ok := m.cache[cfg.SettingID]; ok &&
		cs.storeInfo != nil &&
		cs.storeInfo.PlatformIdentifier() == cfg.PlatformIdentifier {
		m.mu.RUnlock()
		return cs.storePower, nil
	}
	m.mu.RUnlock()
	//	第二级: singleflight 确保同一 settingID 并发时只 build 一次
	v, err, _ := m.sf.Do(cfg.SettingID, func() (any, error) {
		//double-check 缓存
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

// Invalidate 删掉缓存，下次 Resolve 时就会重新 Build。
// 由上层 StorageService 在配置被更新/删除/启禁时调用。
func (m *Manager) Invalidate(settingID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, settingID)
}
