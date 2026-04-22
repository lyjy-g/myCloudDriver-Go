package plugin

import (
	"fmt"
	"sync"
)

// Registry 管理已注册的存储驱动。
type Registry struct {
	mu      sync.RWMutex
	drivers map[PlatformIdentifier]Driver
}

// NewRegistry 创建驱动注册表。
func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[PlatformIdentifier]Driver),
	}
}

// Register 注册一个存储驱动。
func (r *Registry) Register(d Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := d.PlatformIdentifier()
	if _, ok := r.drivers[id]; ok {
		return fmt.Errorf("storage driver already registered: %s", id)
	}

	r.drivers[id] = d
	return nil
}

// MustRegister 注册驱动，失败时直接 panic。
func (r *Registry) MustRegister(d Driver) {
	if err := r.Register(d); err != nil {
		panic(err)
	}
}

// Get 按平台标识获取驱动。
func (r *Registry) Get(id PlatformIdentifier) (Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.drivers[id]
	return d, ok
}
