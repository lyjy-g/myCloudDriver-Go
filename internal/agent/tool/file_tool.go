package tool

import (
	"context"
	"fmt"
	"strings"

	filesvc "myclouddrive-go/internal/file/service"
)

// FileListTool 复用 file service 执行文件检索。
type FileListTool struct {
	svc *filesvc.FileService
}

func NewFileListTool(svc *filesvc.FileService) *FileListTool {
	return &FileListTool{svc: svc}
}

func (t *FileListTool) Name() string { return "tool.file.list" }

func (t *FileListTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	parentID := "root"
	keyword := NormalizeKeyword(callCtx.Query)
	items := t.svc.List(ctx, parentID, keyword, strings.TrimSpace(callCtx.StorageSettingID))
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"fileId":   item.ID,
			"fileName": item.Name,
			"isDir":    item.IsDir,
			"size":     item.Size,
			"updatedAt": item.UpdatedAt,
		})
	}
	return ToolResult{Source: "file", Items: result, Info: fmt.Sprintf("files=%d", len(result))}, nil
}

var _ interface {
	Name() string
	Call(context.Context, CallContext) (ToolResult, error)
} = (*FileListTool)(nil)
