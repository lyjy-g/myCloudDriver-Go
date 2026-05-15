package service // IStoragePower 定义文件模块依赖的最小存储能力集合。
import (
	"context"
	"io"
	storagemodel "myclouddrive-go/internal/storage/model"
	"time"
)

type IStoragePower interface {
	Put(ctx context.Context, in storagemodel.ObjectPutInput) (storagemodel.ObjectInfo, error)
	PresignDownloadURL(ctx context.Context, key string, expire time.Duration) (string, error)
	Get(ctx context.Context, key string) (io.ReadCloser, storagemodel.ObjectInfo, error)
	Delete(ctx context.Context, key string) error
}
