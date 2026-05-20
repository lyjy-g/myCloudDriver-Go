package boot

import (
	"container/list"
	"context"
	"fmt"
	"sync"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/plugin"

	"golang.org/x/sync/singleflight"
)

// cachedStore 表示缓存中的已构建存储实例。
type cachedStore struct {
	cacheKey   string
	settingID  string
	version    string
	storeInfo  plugin.StoreInfo
	storePower plugin.StorePower
	node       *list.Element
}

// Manager 负责存储实例的懒加载、缓存与失效。
// 该结构位于 boot 包，集中承载“插件启动控制”职责。
type Manager struct {
	registry *Registry

	mu sync.RWMutex

	cache map[string]*cachedStore
	lru   *list.List
	cap   int

	sf singleflight.Group
}

const defaultCacheCapacity = 128

// NewManager 创建存储实例管理器。
func NewManager(registry *Registry) *Manager {
	return &Manager{
		registry: registry,
		cache:    make(map[string]*cachedStore),
		lru:      list.New(),
		cap:      defaultCacheCapacity,
	}
}

// Resolve 懒加载的核心: 三级兜底返回 StorePower。
//
//	第一级: 读锁查缓存 → 命中直接返回，并把该缓存项提升到 LRU 前部。
//	第二级: singleflight 确保同一 settingID + version 并发时只 build 一次，并在 build 前 double-check 缓存。
//	第三级: 注册表取出 StoreInfo → Validate → Build（这是"重"操作，如建 S3 client），结果写回缓存。
//
// 缓存 key 直接使用 settingID + version。
// 这样同一个配置被更新后，即使 settingID 不变，也会因为 version 变化命中不到旧实例。
// platformIdentifier 仍保留校验，避免异常数据下串用错误平台实例。
//
// 锁的使用策略是：
// 1. 纯读缓存先走读锁；
// 2. 更新 LRU 顺序和写入缓存时再走写锁；
// 3. Validate / Build 这些重操作放在锁外，只由 singleflight 合并并发。
//
// 长期运行时缓存还会按 LRU 淘汰，避免实例 map 无限增长。
func (m *Manager) Resolve(ctx context.Context, cfg plugin.ResolvedStorageConfig) (plugin.StorePower, error) {
	cacheKey := m.cacheKey(cfg)

	// 第一级: 读锁查缓存 → 命中直接返回。
	m.mu.RLock()
	if cs, ok := m.cache[cacheKey]; ok && m.cacheMatch(cs, cfg) {
		m.mu.RUnlock()
		m.touch(cacheKey)
		return cs.storePower, nil
	}
	m.mu.RUnlock()

	// 第二级: singleflight 确保同一 settingID + version 并发时只 build 一次。
	v, err, _ := m.sf.Do(cacheKey, func() (any, error) {
		// double-check 缓存，避免等待 singleflight 期间其他协程已完成写回。
		m.mu.RLock()
		if cs, ok := m.cache[cacheKey]; ok && m.cacheMatch(cs, cfg) {
			m.mu.RUnlock()
			m.touch(cacheKey)
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
		m.putLocked(&cachedStore{
			cacheKey:   cacheKey,
			settingID:  cfg.SettingID,
			version:    cfg.Version,
			storeInfo:  info,
			storePower: store,
		})
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

	// 一个 settingID 可能已经缓存过多个历史版本，主动失效时要一起清掉。
	keys := make([]string, 0)
	for cacheKey, item := range m.cache {
		if item.settingID == settingID {
			keys = append(keys, cacheKey)
		}
	}
	for _, cacheKey := range keys {
		m.removeLocked(cacheKey)
	}
}

func (m *Manager) cacheMatch(cs *cachedStore, cfg plugin.ResolvedStorageConfig) bool {
	return cs.storeInfo != nil &&
		cs.storeInfo.PlatformIdentifier() == cfg.PlatformIdentifier &&
		cs.version == cfg.Version
}

func (m *Manager) putLocked(cs *cachedStore) {
	if _, ok := m.cache[cs.cacheKey]; ok {
		m.removeLocked(cs.cacheKey)
	}
	cs.node = m.lru.PushFront(cs.cacheKey)
	m.cache[cs.cacheKey] = cs

	// 使用简单 LRU 控制实例数量，避免运行久了后已不再访问的配置实例一直常驻内存。
	// 这里 map 保存缓存项指针，缓存项再反向持有 LRU 节点，这样 touch/remove 都是 O(1)。
	for len(m.cache) > m.cap {
		back := m.lru.Back()
		if back == nil {
			return
		}
		settingID, _ := back.Value.(string)
		m.removeLocked(settingID)
	}
}

func (m *Manager) removeLocked(settingID string) {
	if item, ok := m.cache[settingID]; ok {
		delete(m.cache, settingID)
		if item.node != nil {
			m.lru.Remove(item.node)
			item.node = nil
		}
	}
}

func (m *Manager) touch(settingID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item, ok := m.cache[settingID]; ok && item.node != nil {
		m.lru.MoveToFront(item.node)
	}
}

func (m *Manager) cacheKey(cfg plugin.ResolvedStorageConfig) string {
	return cfg.SettingID + ":" + cfg.Version
}
