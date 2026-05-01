package tool

import (
	"context"
	"fmt"
	"sort"
	"strings"

	filemodel "myclouddrive-go/internal/file/model"
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
	q := strings.TrimSpace(callCtx.Query)
	recentMode := strings.Contains(q, "最近") || strings.Contains(strings.ToLower(q), "recent") || strings.Contains(q, "最新")
	parentID := "root"
	keyword := NormalizeKeyword(q)
	if recentMode {
		// “最近上传”语义应跨目录查询，不应被根目录与关键字过滤误伤。
		parentID = ""
		keyword = ""
	}
	items := t.svc.List(ctx, parentID, keyword, strings.TrimSpace(callCtx.StorageSettingID))
	if recentMode {
		onlyFiles := make([]filemodel.FileItem, 0, len(items))
		for _, it := range items {
			if !it.IsDir {
				onlyFiles = append(onlyFiles, it)
			}
		}
		sort.SliceStable(onlyFiles, func(i, j int) bool { return onlyFiles[i].UpdatedAt.After(onlyFiles[j].UpdatedAt) })
		if len(onlyFiles) > 20 {
			onlyFiles = onlyFiles[:20]
		}
		items = onlyFiles
	}
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"fileId":    item.ID,
			"fileName":  item.Name,
			"isDir":     item.IsDir,
			"size":      item.Size,
			"updatedAt": item.UpdatedAt,
		})
	}
	return ToolResult{Source: "file", Items: result, Info: fmt.Sprintf("files=%d", len(result))}, nil
}

var _ interface {
	Name() string
	Call(context.Context, CallContext) (ToolResult, error)
} = (*FileListTool)(nil)
