package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	agentllm "myclouddrive-go/internal/agent/llm"
	agentmodel "myclouddrive-go/internal/agent/model"
	agentplanner "myclouddrive-go/internal/agent/planner"
	agenttool "myclouddrive-go/internal/agent/tool"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"

	"github.com/google/uuid"
)

// AgentService 是 Agent 编排总入口，负责任务分发。
type AgentService struct {
	registry *agenttool.Registry
	llm      agentllm.Provider
	planner  *agentplanner.Planner
}

func New(registry *agenttool.Registry, llm agentllm.Provider) *AgentService {
	return &AgentService{
		registry: registry,
		llm:      llm,
		planner:  agentplanner.NewPlanner(registry),
	}
}

// Query 统一入口。校验入参 → 鉴权 → scope 解析 → 按 mode 分发。
func (s *AgentService) Query(ctx context.Context, req agentmodel.QueryRequest) (*agentmodel.QueryResponse, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, code.New(code.BadRequest, "query is required")
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "search"
	}
	if mode != "search" && mode != "execute" && mode != "rag" && mode != "workflow" {
		return nil, code.New(code.BadRequest, "invalid mode, allowed: search/execute/rag/workflow")
	}
	if mode != "search" && mode != "execute" {
		return nil, code.New(code.BadRequest, "mode not implemented yet: "+mode)
	}
	if s.registry == nil {
		return nil, code.New(code.InternalError, "agent registry unavailable")
	}
	if s.llm == nil {
		return nil, code.New(code.InternalError, "agent llm unavailable")
	}

	// 鉴权 & 租户隔离
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
	storageSettingID, scope := resolveScope(req.Scope, req.StorageSettingID)

	traceID := strings.TrimSpace(req.TraceID)
	if traceID == "" {
		traceID = "agt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}

	// LLM 决策
	decision, err := s.llm.DecideTools(ctx, req.Query)
	if err != nil {
		log.Printf("agent_step trace=%s step=llm.decide status=error err=%q", traceID, err.Error())
		return nil, code.New(code.InternalError, fmt.Sprintf("llm decide failed: %v", err))
	}
	log.Printf("agent_step trace=%s step=llm.decide status=ok mode=%s scope=%s decision=%+v", traceID, mode, scope, decision)
	intent := strings.TrimSpace(decision.Intent)
	if intent == "" {
		intent = "llm_decision"
	}
	toolNames := decision.Tools
	if len(toolNames) == 0 {
		return nil, code.New(code.BadRequest, "llm returned empty tools")
	}

	callCtx := agenttool.CallContext{
		TraceID:          traceID,
		UserID:           strings.TrimSpace(principal.UserID),
		WorkspaceID:      workspaceID,
		StorageSettingID: storageSettingID,
		Query:            strings.TrimSpace(req.Query),
	}

	// 按 mode 分发
	switch mode {
	case "execute":
		return s.executeMode(ctx, req, traceID, intent, toolNames, scope, callCtx)
	default:
		return s.searchMode(ctx, req, traceID, intent, toolNames, scope, callCtx)
	}
}

// StreamQuery 流式执行，通过 eventFn 逐步回传中间结果。
func (s *AgentService) StreamQuery(ctx context.Context, req agentmodel.QueryRequest, eventFn func(event string, data any)) {
	if strings.TrimSpace(req.Query) == "" {
		eventFn("error", map[string]any{"message": "query is required"})
		return
	}
	eventFn("start", map[string]any{"mode": req.Mode, "query": req.Query})

	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "search"
	}
	principal, err := security.RequireLogin(ctx)
	if err != nil {
		eventFn("error", map[string]any{"message": err.Error()})
		return
	}
	traceID := strings.TrimSpace(req.TraceID)
	if traceID == "" {
		traceID = "agt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	workspaceID := strings.TrimSpace(principal.WorkspaceID)

	eventFn("llm.decide.start", map[string]any{"query": req.Query})
	decision, err := s.llm.DecideTools(ctx, req.Query)
	if err != nil {
		eventFn("llm.decide.error", map[string]any{"error": err.Error()})
		return
	}
	eventFn("llm.decide.done", map[string]any{"intent": decision.Intent, "tools": decision.Tools})

	intent := strings.TrimSpace(decision.Intent)
	if intent == "" {
		intent = "llm_decision"
	}
	toolNames := decision.Tools
	if len(toolNames) == 0 {
		toolNames = []string{"tool.file.list"}
	}

	storageSettingID, _ := resolveScope(req.Scope, req.StorageSettingID)
	callCtx := agenttool.CallContext{
		TraceID:          traceID,
		UserID:           strings.TrimSpace(principal.UserID),
		WorkspaceID:      workspaceID,
		StorageSettingID: storageSettingID,
		Query:            strings.TrimSpace(req.Query),
	}

	if mode == "execute" {
		s.streamExecute(ctx, decision, callCtx, eventFn)
		return
	}
	s.streamSearch(ctx, req.Query, decision, callCtx, eventFn)
}

// resolveScope 解析查询范围。
func resolveScope(scope, storageSettingID string) (string, string) {
	s := strings.TrimSpace(scope)
	id := strings.TrimSpace(storageSettingID)
	if s == "" || s == "auto" {
		if id != "" {
			s = "storage_setting"
		} else {
			s = "workspace"
		}
	}
	if s == "workspace" {
		id = ""
	}
	return id, s
}
