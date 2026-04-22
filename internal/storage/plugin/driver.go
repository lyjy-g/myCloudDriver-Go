package plugin

import "context"

// Driver 定义存储驱动统一能力。
type Driver interface {
	// PlatformIdentifier 返回驱动对应的平台标识。
	PlatformIdentifier() PlatformIdentifier
	// Capabilities 返回驱动支持的能力集合。
	Capabilities() []Capability
	// ValidateConfig 校验原始配置是否合法。
	ValidateConfig(ctx context.Context, raw []byte) error
	// Build 根据解析后的配置构建可用 Store 实例。
	Build(ctx context.Context, cfg ResolvedStorageConfig) (Store, error)
}

// Supports 判断驱动是否支持指定能力。
func Supports(d Driver, target Capability) bool {
	for _, c := range d.Capabilities() {
		if c == target {
			return true
		}
	}
	return false
}
