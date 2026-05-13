package rag

import (
	"context"
	"fmt"
	"io"
	"strings"

	filesvc "myclouddrive-go/internal/file/service"
)

// Loader 从 file service 加载文档。
type Loader struct {
	fileSvc *filesvc.FileService
}

func NewLoader(fileSvc *filesvc.FileService) *Loader {
	return &Loader{fileSvc: fileSvc}
}

// LoadFile 从文件系统加载单个文件内容。
func (l *Loader) LoadFile(ctx context.Context, fileID, storageSettingID string) (*Document, error) {
	rc, _, item, err := l.fileSvc.OpenPreviewContent(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", fileID, err)
	}
	if rc != nil {
		defer rc.Close()
	}
	content := ""
	if rc != nil {
		raw, readErr := io.ReadAll(rc)
		if readErr != nil {
			return nil, fmt.Errorf("read file %s: %w", fileID, readErr)
		}
		content = string(raw)
	}
	return &Document{
		ID:        item.ID,
		FileName:  item.Name,
		FileType:  fileSuffix(item.Name),
		Content:   content,
		Size:      item.Size,
		SourceKey: item.ObjectKey,
		CreatedAt: item.CreatedAt,
	}, nil
}

// LoadFiles 批量加载文件。
func (l *Loader) LoadFiles(ctx context.Context, fileIDs []string, storageSettingID string) ([]Document, error) {
	docs := make([]Document, 0, len(fileIDs))
	for _, id := range fileIDs {
		doc, err := l.LoadFile(ctx, id, storageSettingID)
		if err != nil {
			continue
		}
		docs = append(docs, *doc)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents loaded from %d file IDs", len(fileIDs))
	}
	return docs, nil
}

func fileSuffix(name string) string {
	name = strings.TrimSpace(name)
	idx := strings.LastIndex(name, ".")
	if idx <= 0 || idx >= len(name)-1 {
		return "txt"
	}
	return strings.ToLower(name[idx+1:])
}
