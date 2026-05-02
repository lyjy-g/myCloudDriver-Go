package event

import "sync"

// Bus 是进程内事件总线，解耦 Agent 各阶段之间的硬编码调用链。
type Bus struct {
	mu   sync.RWMutex
	subs map[string][]Handler
}

// Handler 事件处理器。
type Handler func(event string, data any)

// NewBus 创建事件总线。
func NewBus() *Bus {
	return &Bus{subs: make(map[string][]Handler)}
}

// Subscribe 订阅指定事件。
func (b *Bus) Subscribe(event string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[event] = append(b.subs[event], h)
}

// Publish 发布事件，同步通知所有订阅者。
func (b *Bus) Publish(event string, data any) {
	b.mu.RLock()
	handlers := b.subs[event]
	b.mu.RUnlock()
	for _, h := range handlers {
		h(event, data)
	}
}
