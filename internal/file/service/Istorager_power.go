package service // IStoragePower 定义文件模块依赖的最小存储能力集合。
import (
	"context"
	"io"
	storagemodel "myclouddrive-go/internal/storage/model"
	"time"
)

type IStoragePower interface {
	// Put 负责写入对象内容，上传分片和 merge 最终对象都通过它落到底层存储。
	Put(ctx context.Context, in storagemodel.ObjectPutInput) (storagemodel.ObjectInfo, error)
	// PresignDownloadURL 负责生成下载地址，文件模块只消费结果，不关心 local/s3 细节。
	PresignDownloadURL(ctx context.Context, key string, expire time.Duration) (string, error)
	// Get 负责打开对象读取流，预览和流式下载都依赖它。
	Get(ctx context.Context, key string) (io.ReadCloser, storagemodel.ObjectInfo, error)
	// Delete 负责删除底层对象，永久删除和清空回收站都会用到它。
	Delete(ctx context.Context, key string) error
}
