package plugin

import "context"

type Driver interface {
	PlatformIdentifier() PlatformIdentifier
	Capabilities() []Capability
	ValidateConfig(ctx context.Context, raw []byte) error
	Build(ctx context.Context, cfg ResolvedStorageConfig) (Store, error)
}

func Supports(d Driver, target Capability) bool {
	for _, c := range d.Capabilities() {
		if c == target {
			return true
		}
	}
	return false
}
