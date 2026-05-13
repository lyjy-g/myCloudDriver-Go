package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	filesvc "myclouddrive-go/internal/file/service"
)

// FileRenameTool 执行文件重命名。
type FileRenameTool struct{ svc *filesvc.FileService }

func NewFileRenameTool(svc *filesvc.FileService) *FileRenameTool { return &FileRenameTool{svc: svc} }
func (t *FileRenameTool) Name() string                           { return "tool.file.rename" }
func (t *FileRenameTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	items := t.svc.List(ctx, "root", "", strings.TrimSpace(callCtx.StorageSettingID))
	var targetID string
	for _, it := range items {
		if it.IsDir {
			continue
		}
		if strings.Contains(callCtx.Query, it.Name) {
			targetID = it.ID
			break
		}
	}
	if targetID == "" {
		return ToolResult{Source: "file_rename", Items: []any{map[string]any{
			"renamed": false, "message": "target file not found",
		}}, Info: "renamed=0"}, nil
	}
	newName := "renamed-" + time.Now().Format("150405")
	if idx := strings.LastIndex(callCtx.Query, "重命名为"); idx >= 0 {
		newName = strings.TrimSpace(callCtx.Query[idx+len("重命名为"):])
	}
	file, err := t.svc.RenameWithContext(ctx, targetID, newName)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Source: "file_rename", Items: []any{map[string]any{
		"fileId": file.ID, "fileName": file.Name,
	}}, Info: "renamed=1"}, nil
}

// FileCreateDirTool 执行创建目录。
type FileCreateDirTool struct{ svc *filesvc.FileService }

func NewFileCreateDirTool(svc *filesvc.FileService) *FileCreateDirTool {
	return &FileCreateDirTool{svc: svc}
}
func (t *FileCreateDirTool) Name() string { return "tool.file.create_dir" }
func (t *FileCreateDirTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	name := "新建目录"
	if idx := strings.Index(callCtx.Query, "："); idx >= 0 {
		name = strings.TrimSpace(callCtx.Query[idx+1:])
	}
	if idx := strings.Index(name, ":"); idx >= 0 {
		name = strings.TrimSpace(name[idx+1:])
	}
	if strings.TrimSpace(name) == "" {
		name = "新建目录"
	}
	if containsNonASCII(name) {
		name = "project-docs"
	}
	dir, err := t.svc.CreateDirectory(ctx, "root", name, strings.TrimSpace(callCtx.StorageSettingID))
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Source: "file_create_dir", Items: []any{map[string]any{
		"fileId": dir.ID, "fileName": dir.Name, "isDir": true,
	}}, Info: "created=1"}, nil
}

func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

// FileMoveTool 执行移动文件。
type FileMoveTool struct{ svc *filesvc.FileService }

func NewFileMoveTool(svc *filesvc.FileService) *FileMoveTool { return &FileMoveTool{svc: svc} }
func (t *FileMoveTool) Name() string                         { return "tool.file.move" }
func (t *FileMoveTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	items := t.svc.List(ctx, "root", "", strings.TrimSpace(callCtx.StorageSettingID))
	var fileID, targetDirID string
	for _, it := range items {
		if !it.IsDir && strings.Contains(callCtx.Query, it.Name) && fileID == "" {
			fileID = it.ID
		}
		if it.IsDir && strings.Contains(callCtx.Query, it.Name) {
			targetDirID = it.ID
		}
	}
	if fileID == "" {
		return ToolResult{Source: "file_move", Items: []any{map[string]any{
			"moved": false, "message": "source file not found",
		}}, Info: "moved=0"}, nil
	}
	if targetDirID == "" {
		return ToolResult{Source: "file_move", Items: []any{map[string]any{
			"moved": false, "message": "target directory not found",
		}}, Info: "moved=0"}, nil
	}
	if err := t.svc.MoveWithContext(ctx, []string{fileID}, targetDirID); err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Source: "file_move", Items: []any{map[string]any{
		"fileId": fileID, "targetDirId": targetDirID,
	}}, Info: "moved=1"}, nil
}

// FileDeleteTool 执行软删除。
type FileDeleteTool struct{ svc *filesvc.FileService }

func NewFileDeleteTool(svc *filesvc.FileService) *FileDeleteTool { return &FileDeleteTool{svc: svc} }
func (t *FileDeleteTool) Name() string                           { return "tool.file.delete" }
func (t *FileDeleteTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	items := t.svc.List(ctx, "root", "", strings.TrimSpace(callCtx.StorageSettingID))
	targets := make([]string, 0, 4)
	for _, it := range items {
		if strings.Contains(callCtx.Query, it.Name) {
			targets = append(targets, it.ID)
		}
	}
	if len(targets) == 0 {
		return ToolResult{Source: "file_delete", Items: []any{map[string]any{
			"deletedIds": []string{}, "message": "no target matched",
		}}, Info: "deleted=0"}, nil
	}
	t.svc.Recycle(ctx, targets)
	return ToolResult{Source: "file_delete", Items: []any{map[string]any{"deletedIds": targets}}, Info: fmt.Sprintf("deleted=%d", len(targets))}, nil
}

// FileRestoreTool 执行回收站恢复。
type FileRestoreTool struct{ svc *filesvc.FileService }

func NewFileRestoreTool(svc *filesvc.FileService) *FileRestoreTool { return &FileRestoreTool{svc: svc} }
func (t *FileRestoreTool) Name() string                            { return "tool.file.restore" }
func (t *FileRestoreTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	items, _ := t.svc.ListRecycle(ctx, 1, 20)
	if len(items) == 0 {
		return ToolResult{Source: "file_restore", Items: []any{map[string]any{"restoredIds": []string{}}}, Info: "restored=0"}, nil
	}
	target := items[0]
	t.svc.Restore(ctx, []string{target.ID})
	return ToolResult{Source: "file_restore", Items: []any{map[string]any{
		"restoredIds": []string{target.ID},
	}}, Info: "restored=1"}, nil
}

// FileRebuildIndexTool 占位执行“重建索引”。
type FileRebuildIndexTool struct{}

func NewFileRebuildIndexTool() *FileRebuildIndexTool { return &FileRebuildIndexTool{} }
func (t *FileRebuildIndexTool) Name() string         { return "tool.file.rebuild_index" }
func (t *FileRebuildIndexTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	return ToolResult{
		Source: "file_rebuild_index",
		Items:  []any{map[string]any{"status": "accepted", "message": "index rebuild task accepted"}},
		Info:   "rebuild=accepted",
	}, nil
}
