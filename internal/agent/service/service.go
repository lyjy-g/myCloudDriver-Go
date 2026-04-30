package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	agentaudit "myclouddrive-go/internal/agent/audit"
	agentmodel "myclouddrive-go/internal/agent/model"
	agenttool "myclouddrive-go/internal/agent/tool"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"

	"github.com/google/uuid"
)

// AgentService 是检索型 Agent 编排实现。
type AgentService struct {
	registry *agenttool.Registry
	audit    *agentaudit.Logger
}

func New(registry *agenttool.Registry, audit *agentaudit.Logger) *AgentService {
	return &AgentService{registry: registry, audit: audit}
}

func (s *AgentService) EnsureSchema(ctx context.Context) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.EnsureSchema(ctx)
}

func classifyIntent(query string) string {
	q := strings.ToLower(strings.TrimSpace(query))
	switch {
	case strings.Contains(q, "访问"), strings.Contains(q, "记录"), strings.Contains(q, "谁看"), strings.Contains(q, "谁访问"):
		return "share_records"
	case strings.Contains(q, "分享"), strings.Contains(q, "提取码"):
		return "share_list"
	default:
		return "file_list"
	}
}

func mapTools(intent string) []string {
	switch intent {
	case "share_records":
		return []string{"tool.share.records", "tool.share.list"}
	case "share_list":
		return []string{"tool.share.list"}
	default:
		return []string{"tool.file.list"}
	}
}

func summary(intent string, n int, partial bool) string {
	s := fmt.Sprintf("intent=%s 命中 %d 条", intent, n)
	if partial {
		s += "（部分结果）"
	}
	return s
}

func (s *AgentService) Query(ctx context.Context, req agentmodel.QueryRequest) (*agentmodel.QueryResponse, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, code.New(code.BadRequest, "query is required")
	}
	if s.registry == nil {
		return nil, code.New(code.InternalError, "agent registry unavailable")
	}
	principal, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	workspaceID := strings.TrimSpace(principal.WorkspaceID)
	if workspaceID == "" {
		return nil, code.New(code.BadRequest, "workspace required")
	}
	if strings.TrimSpace(req.WorkspaceID) != "" && strings.TrimSpace(req.WorkspaceID) != workspaceID {
		return nil, code.New(code.NoPermission, "workspace mismatch")
	}
	storageSettingID := strings.TrimSpace(req.StorageSettingID)

	traceID := strings.TrimSpace(req.TraceID)
	if traceID == "" {
		traceID = "agt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	intent := classifyIntent(req.Query)
	toolNames := mapTools(intent)

	resp := &agentmodel.QueryResponse{
		TraceID:     traceID,
		Intent:      intent,
		Sources:     []string{},
		Items:       []any{},
		Summary:     "",
		ToolResults: []agentmodel.ToolResult{},
		Partial:     false,
		CreatedAt:   time.Now(),
	}

	callCtx := agenttool.CallContext{
		TraceID:          traceID,
		UserID:           strings.TrimSpace(principal.UserID),
		WorkspaceID:      workspaceID,
		StorageSettingID: storageSettingID,
		Query:            strings.TrimSpace(req.Query),
	}

	for _, toolName := range toolNames {
		if err = s.registry.MustAllowed(toolName); err != nil {
			return nil, code.New(code.NoPermission, err.Error())
		}
		started := time.Now()
		result, callErr := s.registry.Call(ctx, toolName, callCtx, 2*time.Second)
		latency := time.Since(started).Milliseconds()
		tr := agentmodel.ToolResult{Tool: toolName, LatencyMs: latency}
		status := "ok"
		errMsg := ""
		if callErr != nil {
			resp.Partial = true
			status = "error"
			errMsg = callErr.Error()
			tr.Status = status
			tr.Message = errMsg
		} else {
			tr.Status = status
			resp.Sources = append(resp.Sources, result.Source)
			resp.Items = append(resp.Items, result.Items...)
		}
		resp.ToolResults = append(resp.ToolResults, tr)

		if s.audit != nil {
			s.audit.Write(ctx, agentmodel.AuditLog{
				TraceID:          traceID,
				UserID:           principal.UserID,
				WorkspaceID:      workspaceID,
				StorageSettingID: callCtx.StorageSettingID,
				QueryText:        req.Query,
				Intent:           intent,
				ToolName:         toolName,
				Status:           status,
				ErrorMessage:     errMsg,
				LatencyMs:        latency,
				InputSnapshot:    agentaudit.ToInputSnapshot(callCtx),
				OutputSnapshot:   agentaudit.ToOutputSnapshot(result),
			})
		}
	}

	resp.Summary = summary(intent, len(resp.Items), resp.Partial)
	return resp, nil
}
