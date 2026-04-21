package service

import "context"

// PlaceholderService 是 file 服务占位实现。
type PlaceholderService struct{}

func NewPlaceholderService() *PlaceholderService {
	return &PlaceholderService{}
}

func (s *PlaceholderService) Ping(_ context.Context) (string, error) {
	return "file service ready", nil
}
