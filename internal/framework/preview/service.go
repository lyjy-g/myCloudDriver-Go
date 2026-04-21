package preview

import "context"

// Service 表示预览转换能力占位。
type Service interface {
	Convert(ctx context.Context, sourcePath string) (previewPath string, err error)
}
