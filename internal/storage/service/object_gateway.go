package service

import (
	"context"
	"io"
	"myclouddrive-go/internal/storage/plugin"
	"time"
)

// ObjectPutInput 是业务层写入对象入参，不暴露插件底层类型。
type ObjectPutInput struct {
	Key           string
	Reader        io.Reader
	ContentType   string
	ContentLength *int64
	Metadata      map[string]string
}

// ObjectInfo 是业务层对象元信息。
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified *time.Time
}

// Put 将对象写入当前激活存储。
func (s *StorageService) Put(ctx context.Context, in ObjectPutInput) (ObjectInfo, error) {
	info, err := s.PutObject(ctx, plugin.Key(in.Key), in.Reader, plugin.PutOptions{
		ContentType:   in.ContentType,
		ContentLength: in.ContentLength,
		Metadata:      in.Metadata,
	})
	if err != nil {
		return ObjectInfo{}, err
	}
	return toObjectInfo(info), nil
}

// Get 从当前激活存储读取对象。
func (s *StorageService) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	rc, info, err := s.GetObject(ctx, plugin.Key(key))
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return rc, toObjectInfo(info), nil
}

// Delete 删除当前激活存储对象。
func (s *StorageService) Delete(ctx context.Context, key string) error {
	return s.DeleteObject(ctx, plugin.Key(key))
}

// Stat 查询当前激活存储对象元数据。
func (s *StorageService) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	info, err := s.StatObject(ctx, plugin.Key(key))
	if err != nil {
		return ObjectInfo{}, err
	}
	return toObjectInfo(info), nil
}

// PresignDownloadURL 生成下载预签名 URL。
func (s *StorageService) PresignDownloadURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return s.PresignGet(ctx, plugin.Key(key), expire)
}

// PresignUploadURL 生成上传预签名 URL。
func (s *StorageService) PresignUploadURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return s.PresignPut(ctx, plugin.Key(key), expire)
}

func toObjectInfo(info plugin.ObjectInfo) ObjectInfo {
	return ObjectInfo{
		Key:          string(info.Key),
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		LastModified: info.LastModified,
	}
}
