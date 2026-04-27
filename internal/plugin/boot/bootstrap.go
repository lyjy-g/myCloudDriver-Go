package boot

import (
	"myclouddrive-go/internal/plugin/local"
	"myclouddrive-go/internal/plugin/s3"
)

// NewDefaultManager 创建默认插件管理器，并完成系统内置驱动注册。
// 统一放在 boot 包，避免启动逻辑分散在 service 层。
func NewDefaultManager() *Manager {
	registry := NewRegistry()
	registry.Register(local.NewDriver())
	registry.Register(s3.NewDriver())
	return NewManager(registry)
}
