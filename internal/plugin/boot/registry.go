package boot

import (
	"fmt"
	"sync"

	"myclouddrive-go/internal/plugin"
)

// Registry 管理已注册的 StoreInfo。
// 该结构位于 boot 包，用于集中控制插件启动期注册流程。
type Registry struct {
	mu        sync.RWMutex
	storeInfo map[plugin.PlatformIdentifier]plugin.StoreInfo
}

// NewRegistry 创建 StoreInfo 注册表。
func NewRegistry() *Registry {
	return &Registry{
		storeInfo: make(map[plugin.PlatformIdentifier]plugin.StoreInfo),
	}
}

// Register 注册一个 StoreInfo。
func (r *Registry) Register(info plugin.StoreInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := info.PlatformIdentifier()
	if _, ok := r.storeInfo[id]; ok {
		return fmt.Errorf("storage store info already registered: %s", id)
	}

	r.storeInfo[id] = info
	return nil
}

// Get 按平台标识获取 StoreInfo。
func (r *Registry) Get(id plugin.PlatformIdentifier) (plugin.StoreInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.storeInfo[id]
	return info, ok
}
