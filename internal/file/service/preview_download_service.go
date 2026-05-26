package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	filemodel "myclouddrive-go/internal/file/model"
	storagemodel "myclouddrive-go/internal/storage/model"
)

// ResolveDownloadURL 通过统一存储门面生成下载 URL，业务层不关心 local/s3 细节。
func (svc *FileService) ResolveDownloadURL(ctx context.Context, fileID string, expire time.Duration) (string, *filemodel.FileItem, error) {
	// 先拿文件元数据，后续无论走预签名还是回退流式读取都要用到 object_key。
	item, err := svc.Get(ctx, fileID)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(item.ObjectKey) == "" || svc.storage == nil {
		// 没有底层对象 key 或当前没有存储门面时，回退到本服务自己的流式预览接口。
		return fmt.Sprintf("/api/file/stream/preview/%s", fileID), item, nil
	}

	url, err := svc.storage.PresignDownloadURL(ctx, item.ObjectKey, expire)
	if err != nil {
		// 预签名失败时也回退到服务端流式接口，避免把下载能力直接打挂。
		return fmt.Sprintf("/api/file/stream/preview/%s", fileID), item, nil
	}
	return url, item, nil
}

// OpenPreviewContent 通过统一存储门面读取对象内容。
func (svc *FileService) OpenPreviewContent(ctx context.Context, fileID string) (io.ReadCloser, storagemodel.ObjectInfo, *filemodel.FileItem, error) {
	// 预览内容读取同样先依赖文件元数据定位到底层对象。
	item, err := svc.Get(ctx, fileID)
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, nil, err
	}
	if strings.TrimSpace(item.ObjectKey) == "" || svc.storage == nil {
		// 没有对象 key 时只把元数据返回给上层，由上层决定如何处理“不可预览”。
		return nil, storagemodel.ObjectInfo{}, item, nil
	}

	// 真正的内容读取通过统一存储门面下沉，file 模块不关心 local/s3 差异。
	rc, info, err := svc.storage.Get(ctx, item.ObjectKey)
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, item, err
	}
	return rc, info, item, nil
}
