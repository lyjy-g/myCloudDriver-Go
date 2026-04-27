package boot

import (
	"myclouddrive-go/internal/storage/plugin/local"
	"myclouddrive-go/internal/storage/plugin/s3"
)

// NewDefaultManager 创建默认插件管理器，并完成系统内置驱动注册。
// 统一放在 boot 包，避免启动逻辑分散在 service 层。
func NewDefaultManager() *Manager {
	registry := NewRegistry()
	registry.MustRegister(local.NewDriver())
	registry.MustRegister(s3.NewDriver())
	return NewManager(registry)
}
