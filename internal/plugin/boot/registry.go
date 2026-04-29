package boot

import (
	"fmt"
	"sync"

	"myclouddrive-go/internal/plugin"
)

// Registry 管理已注册的存储驱动。
// 该结构位于 boot 包，用于集中控制插件启动期注册流程。
type Registry struct {
	mu        sync.RWMutex
	storeInfo map[plugin.PlatformIdentifier]plugin.StoreInfo
}

// NewRegistry 创建驱动注册表。
func NewRegistry() *Registry {
	return &Registry{
		storeInfo: make(map[plugin.PlatformIdentifier]plugin.StoreInfo),
	}
}

// Register 注册一个存储驱动。
func (r *Registry) Register(d plugin.StoreInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := d.PlatformIdentifier()
	if _, ok := r.storeInfo[id]; ok {
		return fmt.Errorf("storage driver already registered: %s", id)
	}

	r.storeInfo[id] = d
	return nil
}

// Get 按平台标识获取驱动。
func (r *Registry) Get(id plugin.PlatformIdentifier) (plugin.StoreInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.storeInfo[id]
	return d, ok
}
