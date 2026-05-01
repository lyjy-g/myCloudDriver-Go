package tool

import (
	"context"
	"fmt"
	"sort"

	filesvc "myclouddrive-go/internal/file/service"
)

// TransferStatusTool 查询传输任务状态。
type TransferStatusTool struct {
	svc *filesvc.FileService
}

func NewTransferStatusTool(svc *filesvc.FileService) *TransferStatusTool {
	return &TransferStatusTool{svc: svc}
}
func (t *TransferStatusTool) Name() string { return "tool.transfer.status" }

func (t *TransferStatusTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("file service unavailable")
	}
	tasks := t.svc.ListTransferTasks()
	// 按状态分组统计
	statusCount := make(map[string]int)
	for _, task := range tasks {
		statusCount[string(task.Status)]++
	}
	// 返回最近的任务
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
	})
	limit := 10
	if len(tasks) > limit {
		tasks = tasks[:limit]
	}
	items := make([]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, map[string]any{
			"taskId":       task.TaskID,
			"fileName":     task.FileName,
			"fileSize":     task.FileSize,
			"status":       string(task.Status),
			"progress":     fmt.Sprintf("%d/%d", task.UploadedPart, task.TotalParts),
			"uploadedSize": task.UploadedSize,
			"updatedAt":    task.UpdatedAt,
		})
	}
	return ToolResult{
		Source: "transfer",
		Items:  items,
		Info:   fmt.Sprintf("tasks=%d uploading=%d completed=%d", len(tasks), statusCount["UPLOADING"], statusCount["COMPLETED"]),
	}, nil
}
