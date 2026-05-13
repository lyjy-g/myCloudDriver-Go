package cache

import (
	"context"
	"encoding/json"
	"time"
)

// Cache 提供键值缓存抽象。
type Cache interface {
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

// Service 默认进程内缓存实现。
type Service struct {
	store map[string]cacheEntry
}

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

func NewService() *Service {
	return &Service{store: make(map[string]cacheEntry)}
}

func (s *Service) Get(_ context.Context, key string, dest any) error {
	entry, ok := s.store[key]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(s.store, key)
		return ErrNotFound
	}
	return json.Unmarshal(entry.data, dest)
}

func (s *Service) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.store[key] = cacheEntry{data: data, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (s *Service) Del(_ context.Context, key string) error {
	delete(s.store, key)
	return nil
}

var ErrNotFound = &errNotFound{}

type errNotFound struct{}

func (*errNotFound) Error() string { return "cache: not found" }
