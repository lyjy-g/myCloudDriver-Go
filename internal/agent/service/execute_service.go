package service

import (
	"context"
	"fmt"
	"time"

	agentllm "myclouddrive-go/internal/agent/llm"
	agentmodel "myclouddrive-go/internal/agent/model"
	agentplanner "myclouddrive-go/internal/agent/planner"
	agenttool "myclouddrive-go/internal/agent/tool"
	"myclouddrive-go/internal/framework/code"
)

// executeMode 同步 execute 模式：LLM 决策 → Plan → Confirm/执行。
func (s *AgentService) executeMode(ctx context.Context, req agentmodel.QueryRequest,
	traceID, intent string, toolNames []string, scope string, callCtx agenttool.CallContext) (*agentmodel.QueryResponse, error) {

	plan, err := s.planner.BuildPlan(req.Query, intent, toolNames, callCtx)
	if err != nil {
		return nil, code.New(code.InternalError, "build plan failed: "+err.Error())
	}
	resp := &agentmodel.QueryResponse{
		TraceID:   traceID,
		RouteMode: "llm_execute",
		Provider:  s.llm.Name(),
		Model:     s.llm.Model(),
		Intent:    intent,
		Sources:   []string{},
		Items:     []any{map[string]any{"executionPlan": plan}},
		Summary:   plan.Summary,
		Partial:   true,
		CreatedAt: time.Now(),
	}
	if !agentplanner.NeedsConfirmation(plan) {
		resp = s.executeTools(ctx, traceID, toolNames, callCtx, resp)
		resp.Partial = false
		return resp, nil
	}
	// 高风险写操作先缓存待确认计划，避免未确认即执行。
	s.pendingMu.Lock()
	s.pendingPlans[traceID] = &pendingExecution{
		TraceID: traceID,
		Plan:    plan,
		Decision: agentllm.Decision{
			Intent: intent,
			Tools:  toolNames,
		},
		CallCtx: callCtx,
		Query:   req.Query,
		Intent:  intent,
		Mode:    agentmodel.ModeExecute,
	}
	s.pendingMu.Unlock()
	return resp, nil
}

// streamExecute 流式 execute 模式：Plan → Confirm → 执行。
func (s *AgentService) streamExecute(ctx context.Context, decision agentllm.Decision,
	callCtx agenttool.CallContext, eventFn func(string, any), state *agentmodel.StreamState) {

	plan, planErr := s.planner.BuildPlan(callCtx.Query, decision.Intent, decision.Tools, callCtx)
	if planErr != nil {
		eventFn("error", map[string]any{"message": planErr.Error()})
		return
	}
	eventFn("plan", plan)
	if agentplanner.NeedsConfirmation(plan) {
		s.pendingMu.Lock()
		s.pendingPlans[callCtx.TraceID] = &pendingExecution{
			TraceID: callCtx.TraceID,
			Plan:    plan,
			Decision: agentllm.Decision{
				Intent: decision.Intent,
				Tools:  decision.Tools,
			},
			CallCtx: callCtx,
			Query:   callCtx.Query,
			Intent:  decision.Intent,
			Mode:    agentmodel.ModeExecute,
		}
		s.pendingMu.Unlock()
		eventFn("confirm.required", map[string]any{
			"message": "此操作需要确认", "risk": plan.Risk,
			"planId": plan.PlanID,
		})
		return
	}
	// 无风险的 execute 直接执行
	decision2 := agentllm.Decision{Intent: decision.Intent, Tools: decision.Tools}
	s.streamSearch(ctx, callCtx.Query, decision2, callCtx, eventFn, state)
}

// ConfirmExecute 执行 execute 模式中的待确认计划。
func (s *AgentService) ConfirmExecute(ctx context.Context, traceID string) (*agentmodel.QueryResponse, error) {
	s.pendingMu.Lock()
	pending, ok := s.pendingPlans[traceID]
	if ok {
		delete(s.pendingPlans, traceID)
	}
	s.pendingMu.Unlock()
	if !ok || pending == nil {
		return nil, code.New(code.NotFound, "pending execution plan not found: "+traceID)
	}

	resp := &agentmodel.QueryResponse{
		TraceID:     traceID,
		RouteMode:   agentmodel.RouteLLMExecute,
		Provider:    s.llm.Name(),
		Model:       s.llm.Model(),
		Intent:      pending.Intent,
		Sources:     []string{},
		Items:       []any{},
		Summary:     "",
		ToolResults: []agentmodel.ToolResult{},
		Partial:     false,
		CreatedAt:   time.Now(),
	}
	resp = s.executeTools(ctx, traceID, pending.Decision.Tools, pending.CallCtx, resp)
	if len(resp.ToolResults) == 0 {
		resp.Summary = "未执行任何工具，请检查计划与工具配置"
		return resp, nil
	}
	successCount := 0
	for _, tr := range resp.ToolResults {
		if tr.Status == "ok" {
			successCount++
		}
	}
	resp.Partial = successCount != len(resp.ToolResults)
	resp.Summary = fmt.Sprintf("执行完成：成功 %d/%d", successCount, len(resp.ToolResults))
	return resp, nil
}
