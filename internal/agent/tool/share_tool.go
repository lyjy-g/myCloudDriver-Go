package tool

import (
	"context"
	"fmt"
	"strings"

	sharesvc "myclouddrive-go/internal/share/service"
)

// ShareListTool 查询当前工作空间分享列表。
type ShareListTool struct {
	svc *sharesvc.ShareService
}

func NewShareListTool(svc *sharesvc.ShareService) *ShareListTool { return &ShareListTool{svc: svc} }
func (t *ShareListTool) Name() string                            { return "tool.share.list" }

func (t *ShareListTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("share service unavailable")
	}
	items, err := t.svc.ListMyShares(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	out := make([]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"shareId":       it.ShareID,
			"shareName":     it.ShareName,
			"workspaceId":   it.WorkspaceID,
			"storageSetting": it.SettingID,
			"downloadCount": it.DownloadCount,
			"viewCount":     it.ViewCount,
			"status":        it.Status,
		})
	}
	return ToolResult{Source: "share", Items: out, Info: fmt.Sprintf("shares=%d", len(out))}, nil
}

// ShareRecordsTool 查询单个分享访问记录。
type ShareRecordsTool struct {
	svc *sharesvc.ShareService
}

func NewShareRecordsTool(svc *sharesvc.ShareService) *ShareRecordsTool {
	return &ShareRecordsTool{svc: svc}
}
func (t *ShareRecordsTool) Name() string { return "tool.share.records" }

func (t *ShareRecordsTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("share service unavailable")
	}
	query := strings.TrimSpace(callCtx.Query)
	shareID := ""
	if idx := strings.Index(query, "shr_"); idx >= 0 {
		shareID = query[idx:]
		if sp := strings.IndexAny(shareID, " ，,。\n\t"); sp > 0 {
			shareID = shareID[:sp]
		}
	}
	if shareID == "" {
		items, err := t.svc.ListMyShares(ctx)
		if err != nil {
			return ToolResult{}, err
		}
		if len(items) == 0 {
			return ToolResult{Source: "share_records", Items: []any{}, Info: "no-share"}, nil
		}
		shareID = items[0].ShareID
	}
	rows, err := t.svc.ListAccessRecords(ctx, shareID)
	if err != nil {
		return ToolResult{}, err
	}
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"shareId":    row.ShareID,
			"accessIP":   row.AccessIP,
			"browser":    row.Browser,
			"accessTime": row.AccessTime,
		})
	}
	return ToolResult{Source: "share_records", Items: out, Info: fmt.Sprintf("records=%d share=%s", len(out), shareID)}, nil
}
