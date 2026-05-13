package session

import (
	"sync"
	"time"
)

// Manager 管理多轮对话会话。
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewManager(ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &Manager{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
}

// Session 表示一次多轮对话。
type Session struct {
	ID        string
	UserID    string
	Workspace string
	Mode      string
	History   []Message
	Vars      map[string]any
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Message 表示一轮对话。
type Message struct {
	Role    string `json:"role"`    // user / assistant
	Content string `json:"content"` // 消息内容
}

// Create 创建新会话。
func (m *Manager) Create(id, userID, workspace, mode string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	s := &Session{
		ID:        id,
		UserID:    userID,
		Workspace: workspace,
		Mode:      mode,
		History:   make([]Message, 0),
		Vars:      make(map[string]any),
		CreatedAt: now,
		ExpiresAt: now.Add(m.ttl),
	}
	m.sessions[id] = s
	return s
}

// Get 获取会话，同时更新过期时间。
func (m *Manager) Get(id string) *Session {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	m.mu.Lock()
	s.ExpiresAt = time.Now().Add(m.ttl)
	m.mu.Unlock()
	return s
}

// Delete 删除会话。
func (m *Manager) Delete(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

// List 列出用户的所有活跃会话。
func (m *Manager) List(userID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Session, 0)
	for _, s := range m.sessions {
		if s.UserID == userID && time.Now().Before(s.ExpiresAt) {
			result = append(result, s)
		}
	}
	return result
}
