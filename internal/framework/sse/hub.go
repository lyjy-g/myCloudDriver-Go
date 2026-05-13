package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Event 表示一条 SSE 事件。
type Event struct {
	Event string `json:"event,omitempty"`
	Data  any    `json:"data"`
}

// Broker 管理 SSE 连接。
type Broker struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[chan Event]struct{}),
	}
}

// ServeHTTP 实现 http.Handler，建立 SSE 连接。
func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := make(chan Event, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()

	notify := r.Context().Done()
	go func() {
		<-notify
		b.mu.Lock()
		delete(b.clients, ch)
		b.mu.Unlock()
	}()

	for {
		select {
		case <-notify:
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			writeEvent(w, evt)
			flusher.Flush()
		}
	}
}

// Send 向所有连接的客户端发送事件。仅用于全局广播（系统状态变更），
// 禁止在此写入用户级敏感数据——这会跨用户泄漏。
func (b *Broker) Send(evt Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- evt:
		default:
		}
	}
}

// SendTo 向指定 channel 发送事件（用于 per-request 流）。
func SendTo(ch chan Event, evt Event) {
	select {
	case ch <- evt:
	default:
	}
}

func writeEvent(w http.ResponseWriter, evt Event) {
	if evt.Event != "" {
		fmt.Fprintf(w, "event: %s\n", evt.Event)
	}
	data, err := json.Marshal(evt.Data)
	if err != nil {
		data = []byte(`{"error":"marshal failed"}`)
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// NewResponseWriter 创建 per-request SSE 响应 writer。
func NewResponseWriter(w http.ResponseWriter) (chan Event, func()) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, nil
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := make(chan Event, 64)
	done := make(chan struct{})

	go func() {
		defer func() { recover() }() // 防止 handler 返回后 flush 已关闭的连接
		for {
			evt, ok := <-ch
			if !ok {
				return
			}
			writeEvent(w, evt)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}()

	cancel := func() {
		select {
		case <-done:
		default:
			close(done)
			close(ch)
		}
	}
	return ch, cancel
}
