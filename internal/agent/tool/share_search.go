package tool

import (
	"context"
	"fmt"
	"sort"
	"strings"

	agentmodel "myclouddrive-go/internal/agent/model"
	sharesvc "myclouddrive-go/internal/share/service"
)

// ShareSearchTool 查询我的分享链接（带过滤）。
// 已废弃：用户已将 tool.share.search 合并到统一分类中，但此处保留以支持 LLM 路由。
type ShareSearchTool struct {
	svc *sharesvc.ShareService
}

func NewShareSearchTool(svc *sharesvc.ShareService) *ShareSearchTool {
	return &ShareSearchTool{svc: svc}
}
func (t *ShareSearchTool) Name() string { return "tool.share.search" }

func (t *ShareSearchTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("share service unavailable")
	}
	items, err := t.svc.ListMyShares(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	// 按下载/查看次数排序，优先展示高热度分享
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].DownloadCount+items[i].ViewCount > items[j].DownloadCount+items[j].ViewCount
	})
	out := make([]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"shareId":       it.ShareID,
			"shareName":     it.ShareName,
			"workspaceId":   it.WorkspaceID,
			"downloadCount": it.DownloadCount,
			"viewCount":     it.ViewCount,
			"status":        it.Status,
		})
	}
	return ToolResult{Source: "share", Items: out, Info: fmt.Sprintf("shares=%d", len(out))}, nil
}

// ShareStatsTool 分享统计聚合。
type ShareStatsTool struct {
	svc *sharesvc.ShareService
}

func NewShareStatsTool(svc *sharesvc.ShareService) *ShareStatsTool {
	return &ShareStatsTool{svc: svc}
}
func (t *ShareStatsTool) Name() string { return "tool.share.stats" }

func (t *ShareStatsTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("share service unavailable")
	}
	items, err := t.svc.ListMyShares(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	totalViews := 0
	totalDownloads := 0
	topByViews := make([]agentmodel.ShareStatEntry, 0)
	topByDownloads := make([]agentmodel.ShareStatEntry, 0)
	statusCount := map[int]int{0: 0, 1: 0}

	for _, it := range items {
		totalViews += it.ViewCount
		totalDownloads += it.DownloadCount
		statusCount[it.Status]++
		topByViews = append(topByViews, agentmodel.ShareStatEntry{ShareID: it.ShareID, ShareName: it.ShareName, Value: it.ViewCount})
		topByDownloads = append(topByDownloads, agentmodel.ShareStatEntry{ShareID: it.ShareID, ShareName: it.ShareName, Value: it.DownloadCount})
	}
	sort.SliceStable(topByViews, func(i, j int) bool { return topByViews[i].Value > topByViews[j].Value })
	sort.SliceStable(topByDownloads, func(i, j int) bool { return topByDownloads[i].Value > topByDownloads[j].Value })
	if len(topByViews) > 5 {
		topByViews = topByViews[:5]
	}
	if len(topByDownloads) > 5 {
		topByDownloads = topByDownloads[:5]
	}

	stats := map[string]any{
		"totalShares":    len(items),
		"totalViews":     totalViews,
		"totalDownloads": totalDownloads,
		"activeCount":    statusCount[0],
		"expiredCount":   statusCount[1],
		"topByViews":     topByViews,
		"topByDownloads": topByDownloads,
	}
	q := strings.TrimSpace(callCtx.Query)
	stats["queryHint"] = q
	out := []any{stats}
	return ToolResult{Source: "share_stats", Items: out, Info: fmt.Sprintf("shares=%d views=%d downloads=%d", len(items), totalViews, totalDownloads)}, nil
}

// ShareRevokeTool 撤销分享。
type ShareRevokeTool struct {
	svc *sharesvc.ShareService
}

func NewShareRevokeTool(svc *sharesvc.ShareService) *ShareRevokeTool {
	return &ShareRevokeTool{svc: svc}
}
func (t *ShareRevokeTool) Name() string { return "tool.share.revoke" }

func (t *ShareRevokeTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil {
		return ToolResult{}, fmt.Errorf("share service unavailable")
	}
	// 从 query 中提取 shareID，或撤销所有分享
	q := strings.TrimSpace(callCtx.Query)
	var shareIDs []string
	if idx := strings.Index(q, "shr_"); idx >= 0 {
		shareID := q[idx:]
		if sp := strings.IndexAny(shareID, " ，,。\n\t"); sp > 0 {
			shareID = shareID[:sp]
		}
		shareIDs = []string{shareID}
	}
	if len(shareIDs) == 0 {
		shares, err := t.svc.ListMyShares(ctx)
		if err != nil {
			return ToolResult{}, err
		}
		if len(shares) == 0 {
			return ToolResult{}, fmt.Errorf("no share available to revoke")
		}
		// 默认撤销最近创建的一条，匹配“撤销最近创建的一条分享”这类自然语言。
		shareIDs = []string{shares[0].ShareID}
	}
	if err := t.svc.CancelShares(ctx, shareIDs); err != nil {
		return ToolResult{}, err
	}
	out := []any{map[string]any{"revokedShareIds": shareIDs}}
	return ToolResult{Source: "share_revoke", Items: out, Info: fmt.Sprintf("revoked=%d", len(shareIDs))}, nil
}
