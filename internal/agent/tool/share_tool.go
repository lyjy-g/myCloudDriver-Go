package tool

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	filesvc "myclouddrive-go/internal/file/service"
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
			"shareId":        it.ShareID,
			"shareName":      it.ShareName,
			"workspaceId":    it.WorkspaceID,
			"storageSetting": it.SettingID,
			"downloadCount":  it.DownloadCount,
			"viewCount":      it.ViewCount,
			"status":         it.Status,
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

// ShareCreateTool 创建分享。
type ShareCreateTool struct {
	svc     *sharesvc.ShareService
	fileSvc *filesvc.FileService
}

func NewShareCreateTool(svc *sharesvc.ShareService, fileSvc *filesvc.FileService) *ShareCreateTool {
	return &ShareCreateTool{svc: svc, fileSvc: fileSvc}
}
func (t *ShareCreateTool) Name() string { return "tool.share.create" }

func (t *ShareCreateTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.svc == nil || t.fileSvc == nil {
		return ToolResult{}, fmt.Errorf("share service unavailable")
	}
	q := strings.TrimSpace(callCtx.Query)
	fileIDs := extractFileIDs(q)
	if len(fileIDs) == 0 {
		fileIDs = pickDefaultShareFileIDs(ctx, t.fileSvc, callCtx.StorageSettingID, 1)
	}
	if len(fileIDs) == 0 {
		return ToolResult{}, fmt.Errorf("no file available for share creation")
	}
	expireSeconds := int64(24 * 3600)
	if h := extractExpireHours(q); h > 0 {
		expireSeconds = int64(h * 3600)
	}
	allowDownload := !strings.Contains(q, "不允许下载") && !strings.Contains(strings.ToLower(q), "read only")
	shareName := "Agent 自动创建分享"
	if strings.Contains(q, "分享") {
		shareName = strings.TrimSpace(q)
	}
	share, err := t.svc.CreateShare(ctx, sharesvc.CreateShareReq{
		ShareName:     shareName,
		FileIDs:       fileIDs,
		ExpireSeconds: &expireSeconds,
		AllowDownload: &allowDownload,
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Source: "share_create",
		Items: []any{map[string]any{
			"shareId":        share.ShareID,
			"shareName":      share.ShareName,
			"shareCode":      share.ShareCode,
			"allowDownload":  share.AllowDownload,
			"expireTime":     share.ExpireTime,
			"workspaceId":    share.WorkspaceID,
			"storageSetting": share.SettingID,
			"fileIds":        share.FileIDs,
		}},
		Info: fmt.Sprintf("created_share=%s files=%d", share.ShareID, len(fileIDs)),
	}, nil
}

func extractFileIDs(query string) []string {
	re := regexp.MustCompile(`f_[a-zA-Z0-9]+`)
	return dedupeStringSlice(re.FindAllString(query, -1))
}

func extractExpireHours(query string) int {
	re := regexp.MustCompile(`(\d+)\s*小时`)
	matches := re.FindStringSubmatch(query)
	if len(matches) < 2 {
		return 0
	}
	h, err := strconv.Atoi(strings.TrimSpace(matches[1]))
	if err != nil || h <= 0 {
		return 0
	}
	return h
}

func pickDefaultShareFileIDs(ctx context.Context, fileSvc *filesvc.FileService, settingID string, limit int) []string {
	items := fileSvc.List(ctx, "root", "", strings.TrimSpace(settingID))
	out := make([]string, 0, limit)
	for _, it := range items {
		if it.IsDir {
			continue
		}
		out = append(out, it.ID)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func dedupeStringSlice(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
