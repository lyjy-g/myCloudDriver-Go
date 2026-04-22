package plugin

import (
	"fmt"
	"sync"
)

type Registry struct {
	mu      sync.RWMutex
	drivers map[PlatformIdentifier]Driver
}

func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[PlatformIdentifier]Driver),
	}
}

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

func (r *Registry) MustRegister(d Driver) {
	if err := r.Register(d); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(id PlatformIdentifier) (Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.drivers[id]
	return d, ok
}
