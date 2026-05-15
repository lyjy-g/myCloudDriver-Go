package service

import (
	"context"
	"io"
	pluginsvc "myclouddrive-go/internal/plugin/service"
	dto "myclouddrive-go/internal/storage/model"
	"time"
)

// Put 将对象写入当前激活存储。
//
// 可忽略：
// - 该函数主要做入参映射，不承担复杂业务决策。
func (s *StorageService) Put(ctx context.Context, in dto.ObjectPutInput) (dto.ObjectInfo, error) {
	info, err := s.runManager.Put(ctx, in.Key, in.Reader, pluginsvc.PutOptions{
		ContentType:   in.ContentType,
		ContentLength: in.ContentLength,
		Metadata:      in.Metadata,
	})
	if err != nil {
		return dto.ObjectInfo{}, err
	}
	return toObjectInfo(info), nil
}

// Get 从当前激活存储读取对象。
func (s *StorageService) Get(ctx context.Context, key string) (io.ReadCloser, dto.ObjectInfo, error) {
	rc, info, err := s.runManager.Get(ctx, key)
	if err != nil {
		return nil, dto.ObjectInfo{}, err
	}
	return rc, toObjectInfo(info), nil
}

// GetBySetting 使用指定存储配置读取对象。
//
// 面试可讲：
// - 分享/审计等跨页面场景必须“按资源归属配置读”，不能依赖当前激活配置。
func (s *StorageService) GetBySetting(ctx context.Context, settingID string, key string) (io.ReadCloser, dto.ObjectInfo, error) {
	rc, info, err := s.runManager.GetBySetting(ctx, settingID, key)
	if err != nil {
		return nil, dto.ObjectInfo{}, err
	}
	return rc, toObjectInfo(info), nil
}

// Delete 删除当前激活存储对象。
func (s *StorageService) Delete(ctx context.Context, key string) error {
	return s.runManager.Delete(ctx, key)
}

// Stat 查询当前激活存储对象元数据。
func (s *StorageService) Stat(ctx context.Context, key string) (dto.ObjectInfo, error) {
	info, err := s.runManager.Stat(ctx, key)
	if err != nil {
		return dto.ObjectInfo{}, err
	}
	return toObjectInfo(info), nil
}

// PresignDownloadURL 生成下载预签名 URL。
//
// 面试可讲：
// - file 模块只依赖该门面，不感知 local/s3 的签名实现差异。
func (s *StorageService) PresignDownloadURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return s.runManager.PresignGet(ctx, key, expire)
}

// PresignUploadURL 生成上传预签名 URL。
func (s *StorageService) PresignUploadURL(ctx context.Context, key string, expire time.Duration) (string, error) {
	return s.runManager.PresignPut(ctx, key, expire)
}
