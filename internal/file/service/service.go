package service

import "context"

// Service 定义 file 服务能力（占位）。
type Service interface {
	Ping(ctx context.Context) (string, error)
}

// PlaceholderService 是 file 服务占位实现。
type PlaceholderService struct{}

func NewPlaceholderService() Service {
	return &PlaceholderService{}
}

func (s *PlaceholderService) Ping(_ context.Context) (string, error) {
	return "file service ready", nil
}
