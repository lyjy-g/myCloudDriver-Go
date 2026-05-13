package plugin

import "context"

// StoreInfo 定义存储平台信息能力。
type StoreInfo interface {
	// PlatformIdentifier 返回平台标识。
	PlatformIdentifier() PlatformIdentifier
	// Capabilities 返回平台支持的能力集合。
	Capabilities() []Capability
	// ValidateConfig 校验原始配置是否合法。
	ValidateConfig(ctx context.Context, raw []byte) error
	// Build 根据解析后的配置构建可用 StorePower 实例。
	Build(ctx context.Context, cfg ResolvedStorageConfig) (StorePower, error)
}

// Supports 判断 StoreInfo 是否支持指定能力。
func Supports(info StoreInfo, target Capability) bool {
	for _, c := range info.Capabilities() {
		if c == target {
			return true
		}
	}
	return false
}
