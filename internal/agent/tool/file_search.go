package tool

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	filemodel "myclouddrive-go/internal/file/model"
	filesvc "myclouddrive-go/internal/file/service"
)

// FileSearchTool 通用文件检索（带过滤条件）。
type FileSearchTool struct {
	svc *filesvc.FileService
}

func NewFileSearchTool(svc *filesvc.FileService) *FileSearchTool { return &FileSearchTool{svc: svc} }
func (t *FileSearchTool) Name() string                           { return "tool.file.search" }

func (t *FileSearchTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	q := strings.TrimSpace(callCtx.Query)
	parentID := "root"
	keyword := NormalizeKeyword(q)
	if strings.Contains(q, "最近") || strings.Contains(q, "最新") {
		parentID = ""
		keyword = ""
	}
	items := t.svc.List(ctx, parentID, keyword, strings.TrimSpace(callCtx.StorageSettingID))
	if strings.Contains(q, "最近") || strings.Contains(q, "最新") {
		items = filterRecent(items, 20)
	}
	// 按文件类型过滤
	if fileType := extractFileType(q); fileType != "" {
		items = filterByType(items, fileType)
	}
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"fileId":    item.ID,
			"fileName":  item.Name,
			"isDir":     item.IsDir,
			"size":      item.Size,
			"updatedAt": item.UpdatedAt,
			"createdAt": item.CreatedAt,
		})
	}
	return ToolResult{Source: "file", Items: result, Info: fmt.Sprintf("files=%d", len(result))}, nil
}

// FileStatsTool 文件统计聚合。
type FileStatsTool struct {
	svc *filesvc.FileService
}

func NewFileStatsTool(svc *filesvc.FileService) *FileStatsTool { return &FileStatsTool{svc: svc} }
func (t *FileStatsTool) Name() string                          { return "tool.file.stats" }

func (t *FileStatsTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	items := t.svc.List(ctx, "", "", strings.TrimSpace(callCtx.StorageSettingID))
	onlyFiles := make([]filemodel.FileItem, 0, len(items))
	for _, it := range items {
		if !it.IsDir {
			onlyFiles = append(onlyFiles, it)
		}
	}
	q := strings.TrimSpace(callCtx.Query)
	fileType := extractFileType(q)

	// 按文件类型统计
	typeStats := make(map[string]int)
	// 按大小分桶
	var sizeBuckets = map[string]int{"<1MB": 0, "1-10MB": 0, "10-100MB": 0, ">100MB": 0}
	totalSize := int64(0)
	for _, it := range onlyFiles {
		suffix := strings.ToLower(fileSuffix(it.Name))
		if suffix == "" {
			suffix = "other"
		}
		typeStats[suffix]++
		totalSize += it.Size
		switch {
		case it.Size < 1024*1024:
			sizeBuckets["<1MB"]++
		case it.Size < 10*1024*1024:
			sizeBuckets["1-10MB"]++
		case it.Size < 100*1024*1024:
			sizeBuckets["10-100MB"]++
		default:
			sizeBuckets[">100MB"]++
		}
	}
	stats := map[string]any{
		"totalFiles":   len(onlyFiles),
		"totalSize":    totalSize,
		"byType":       typeStats,
		"bySizeBucket": sizeBuckets,
	}
	if fileType != "" {
		stats["filterType"] = fileType
		stats["filterTypeCount"] = typeStats[fileType]
	}
	out := []any{stats}
	return ToolResult{Source: "file_stats", Items: out, Info: fmt.Sprintf("files=%d", len(onlyFiles))}, nil
}

// FileTrashListTool 回收站列表。
type FileTrashListTool struct {
	svc *filesvc.FileService
}

func NewFileTrashListTool(svc *filesvc.FileService) *FileTrashListTool {
	return &FileTrashListTool{svc: svc}
}
func (t *FileTrashListTool) Name() string { return "tool.file.trash.list" }

func (t *FileTrashListTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	items, _ := t.svc.ListRecycle(1, 100)
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"fileId":    item.ID,
			"fileName":  item.Name,
			"isDir":     item.IsDir,
			"size":      item.Size,
			"updatedAt": item.UpdatedAt,
			"deletedAt": item.DeletedAt,
		})
	}
	return ToolResult{Source: "trash", Items: result, Info: fmt.Sprintf("trashed=%d", len(result))}, nil
}

// FileRankTool 重要文件排序（面试亮点）。
type FileRankTool struct {
	svc *filesvc.FileService
}

func NewFileRankTool(svc *filesvc.FileService) *FileRankTool { return &FileRankTool{svc: svc} }
func (t *FileRankTool) Name() string                         { return "tool.file.rank" }

func (t *FileRankTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	items := t.svc.List(ctx, "", "", strings.TrimSpace(callCtx.StorageSettingID))
	onlyFiles := make([]filemodel.FileItem, 0, len(items))
	for _, it := range items {
		if !it.IsDir {
			onlyFiles = append(onlyFiles, it)
		}
	}
	now := time.Now()
	type ranked struct {
		filemodel.FileItem
		score float64
	}
	rankList := make([]ranked, 0, len(onlyFiles))
	for _, it := range onlyFiles {
		// importance score: recency * 0.4 + size_factor * 0.3 + fileTypeWeight * 0.2 + ageDecay
		ageHours := now.Sub(it.UpdatedAt).Hours()
		recency := math.Max(0, 100-math.Min(ageHours, 100)) * 0.4
		sizeScore := math.Log(float64(it.Size+1)) * 0.3
		typeWeight := 0.0
		switch strings.ToLower(fileSuffix(it.Name)) {
		case "pdf", "doc", "docx", "xlsx":
			typeWeight = 0.2
		case "png", "jpg", "jpeg", "mp4":
			typeWeight = 0.1
		default:
			typeWeight = 0.05
		}
		score := recency + sizeScore*2 + typeWeight*10
		rankList = append(rankList, ranked{FileItem: it, score: score})
	}
	sort.Slice(rankList, func(i, j int) bool { return rankList[i].score > rankList[j].score })
	if len(rankList) > 10 {
		rankList = rankList[:10]
	}
	result := make([]any, 0, len(rankList))
	for _, r := range rankList {
		result = append(result, map[string]any{
			"fileId":          r.ID,
			"fileName":        r.Name,
			"size":            r.Size,
			"updatedAt":       r.UpdatedAt,
			"createdAt":       r.CreatedAt,
			"importanceScore": fmt.Sprintf("%.1f", r.score)},
		)
	}
	return ToolResult{Source: "file_rank", Items: result, Info: fmt.Sprintf("ranked=%d", len(result))}, nil
}

func filterRecent(items []filemodel.FileItem, limit int) []filemodel.FileItem {
	onlyFiles := make([]filemodel.FileItem, 0)
	for _, it := range items {
		if !it.IsDir {
			onlyFiles = append(onlyFiles, it)
		}
	}
	sort.SliceStable(onlyFiles, func(i, j int) bool { return onlyFiles[i].UpdatedAt.After(onlyFiles[j].UpdatedAt) })
	if len(onlyFiles) > limit {
		onlyFiles = onlyFiles[:limit]
	}
	return onlyFiles
}

func extractFileType(query string) string {
	q := strings.ToLower(query)
	for _, t := range []string{"pdf", "doc", "docx", "xlsx", "pptx", "png", "jpg", "jpeg", "mp4", "txt", "zip"} {
		if strings.Contains(q, t) {
			return t
		}
	}
	return ""
}

func filterByType(items []filemodel.FileItem, fileType string) []filemodel.FileItem {
	result := make([]filemodel.FileItem, 0)
	for _, it := range items {
		if it.IsDir {
			continue
		}
		if strings.EqualFold(fileSuffix(it.Name), fileType) {
			result = append(result, it)
		}
	}
	return result
}

func fileSuffix(name string) string {
	name = strings.TrimSpace(name)
	idx := strings.LastIndex(name, ".")
	if idx <= 0 || idx >= len(name)-1 {
		return ""
	}
	return strings.ToLower(name[idx+1:])
}
