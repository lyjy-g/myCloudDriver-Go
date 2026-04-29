package boot

import (
	"myclouddrive-go/internal/plugin/local"
	"myclouddrive-go/internal/plugin/s3"
)

// NewDefaultManager 创建默认插件管理器，并完成系统内置 StoreInfo 注册。
// 统一放在 boot 包，避免启动逻辑分散在 service 层。
func NewDefaultManager() *Manager {
	registry := NewRegistry()
	err := registry.Register(local.NewStoreInfo())
	if err != nil {
		return nil
	}
	err = registry.Register(s3.NewStoreInfo())
	if err != nil {
		return nil
	}
	return NewManager(registry)
}
