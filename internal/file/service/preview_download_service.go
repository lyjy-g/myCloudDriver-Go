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
	item, err := svc.Get(ctx, fileID)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(item.ObjectKey) == "" || svc.storage == nil {
		return fmt.Sprintf("/api/file/stream/preview/%s", fileID), item, nil
	}

	url, err := svc.storage.PresignDownloadURL(ctx, item.ObjectKey, expire)
	if err != nil {
		return fmt.Sprintf("/api/file/stream/preview/%s", fileID), item, nil
	}
	return url, item, nil
}

// OpenPreviewContent 通过统一存储门面读取对象内容。
func (svc *FileService) OpenPreviewContent(ctx context.Context, fileID string) (io.ReadCloser, storagemodel.ObjectInfo, *filemodel.FileItem, error) {
	item, err := svc.Get(ctx, fileID)
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, nil, err
	}
	if strings.TrimSpace(item.ObjectKey) == "" || svc.storage == nil {
		return nil, storagemodel.ObjectInfo{}, item, nil
	}

	rc, info, err := svc.storage.Get(ctx, item.ObjectKey)
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, item, err
	}
	return rc, info, item, nil
}
