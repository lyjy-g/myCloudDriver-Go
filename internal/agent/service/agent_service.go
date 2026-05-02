package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	agenthistory "myclouddrive-go/internal/agent/history"
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
	history  *agenthistory.Service
	runSvc   *runService

	// 流式查询取消管理
	streamCancel map[string]context.CancelFunc
	streamState  map[string]*agentmodel.StreamState
	streamMu     sync.Mutex

	// execute 模式待确认计划缓存（traceId -> pending）
	pendingMu    sync.Mutex
	pendingPlans map[string]*pendingExecution
}

type pendingExecution struct {
	TraceID  string
	Plan     agentmodel.ExecutionPlan
	Decision agentllm.Decision
	CallCtx  agenttool.CallContext
	Query    string
	Intent   string
	Mode     string
}

func New(registry *agenttool.Registry, llm agentllm.Provider, history *agenthistory.Service) *AgentService {
	return &AgentService{
		registry:     registry,
		llm:          llm,
		planner:      agentplanner.NewPlanner(registry),
		history:      history,
		runSvc:       newRunService(),
		streamCancel: make(map[string]context.CancelFunc),
		streamState:  make(map[string]*agentmodel.StreamState),
		pendingPlans: make(map[string]*pendingExecution),
	}
}

// StopStream 取消正在流式执行的查询并持久化已产生的部分结果。
func (s *AgentService) StopStream(ctx context.Context, traceID string) error {
	s.streamMu.Lock()
	cancel, ok := s.streamCancel[traceID]
	state := s.streamState[traceID]
	delete(s.streamCancel, traceID)
	delete(s.streamState, traceID)
	s.streamMu.Unlock()

	if !ok {
		return code.New(code.NotFound, "stream not found: "+traceID)
	}

	cancel() // 触发流式 goroutine 中的 context 取消

	// 持久化部分结果
	if state != nil && state.Dirty && s.history != nil {
		entry := &agenthistory.Entry{
			TraceID:   state.TraceID,
			Query:     state.Query,
			Summary:   state.Summary,
			Intent:    state.Intent,
			Mode:      state.Mode,
			ItemCount: state.ItemCount,
			CreatedAt: time.Now(),
		}
		_ = s.history.Record(context.Background(), state.UserID, state.WorkspaceID, entry)
	}
	return nil
}

// ListHistory 返回最近的 n 条对话历史。beforeTraceID 不为空时翻页。
func (s *AgentService) ListHistory(ctx context.Context, userID, workspaceID, beforeTraceID string, n int) ([]agenthistory.Entry, bool, error) {
	if s.history == nil {
		return nil, false, nil
	}
	entries, err := s.history.List(ctx, userID, workspaceID, beforeTraceID, n)
	if err != nil {
		return nil, false, err
	}
	hasMore := false
	if len(entries) > 0 {
		lastID := entries[len(entries)-1].TraceID
		hasMore, _ = s.history.HasMore(ctx, userID, workspaceID, lastID)
	}
	return entries, hasMore, nil
}

// recordHistory 将本次查询写入历史。
func (s *AgentService) recordHistory(userID, workspaceID string, req agentmodel.QueryRequest, resp *agentmodel.QueryResponse) {
	if s.history == nil || resp == nil {
		return
	}
	source := ""
	if len(resp.Sources) > 0 {
		source = resp.Sources[0]
	}
	entry := &agenthistory.Entry{
		TraceID:   resp.TraceID,
		Query:     req.Query,
		Summary:   resp.Summary,
		Intent:    resp.Intent,
		Mode:      req.Mode,
		Source:    source,
		ItemCount: len(resp.Items),
		CreatedAt: time.Now(),
	}
	_ = s.history.Record(context.Background(), userID, workspaceID, entry)
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

	//占位符
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

	callCtx := agenttool.CallContext{
		TraceID:          traceID,
		UserID:           strings.TrimSpace(principal.UserID),
		WorkspaceID:      workspaceID,
		StorageSettingID: storageSettingID,
		Query:            strings.TrimSpace(req.Query),
	}

	// 按 mode 分发
	var resp *agentmodel.QueryResponse
	switch mode {
	case "execute":
		resp, err = s.executeMode(ctx, req, traceID, intent, toolNames, scope, callCtx)
	default:
		resp, err = s.searchMode(ctx, req, traceID, intent, toolNames, scope, callCtx)
	}
	if err == nil && resp != nil {
		s.recordHistory(principal.UserID, workspaceID, req, resp)
	}
	return resp, err
}

// StreamQuery 流式执行，通过 eventFn 逐步回传中间结果。
func (s *AgentService) StreamQuery(ctx context.Context, req agentmodel.QueryRequest, eventFn func(event string, data any)) {
	if strings.TrimSpace(req.Query) == "" {
		eventFn("error", map[string]any{"message": "query is required"})
		return
	}
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
	userID := strings.TrimSpace(principal.UserID)

	// 创建可取消 context 并注册到 stream 管理
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	state := &agentmodel.StreamState{
		TraceID:     traceID,
		UserID:      userID,
		WorkspaceID: workspaceID,
		Query:       strings.TrimSpace(req.Query),
		Mode:        mode,
	}
	s.streamMu.Lock()
	s.streamCancel[traceID] = cancel
	s.streamState[traceID] = state
	s.streamMu.Unlock()
	defer func() {
		s.streamMu.Lock()
		delete(s.streamCancel, traceID)
		delete(s.streamState, traceID)
		s.streamMu.Unlock()
	}()

	eventFn("start", map[string]any{"mode": req.Mode, "query": req.Query, "traceId": traceID, "provider": s.llm.Name(), "model": s.llm.Model()})

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
	state.Intent = intent

	storageSettingID, _ := resolveScope(req.Scope, req.StorageSettingID)
	callCtx := agenttool.CallContext{
		TraceID:          traceID,
		UserID:           userID,
		WorkspaceID:      workspaceID,
		StorageSettingID: storageSettingID,
		Query:            strings.TrimSpace(req.Query),
	}

	if mode == "execute" {
		s.streamExecute(ctx, decision, callCtx, eventFn, state)
		return
	}
	s.streamSearch(ctx, req.Query, decision, callCtx, eventFn, state)
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
