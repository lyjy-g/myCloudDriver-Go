package service

import (
	"context"
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
	}
	return resp, nil
}

// streamExecute 流式 execute 模式：Plan → Confirm → 执行。
func (s *AgentService) streamExecute(ctx context.Context, decision agentllm.Decision,
	callCtx agenttool.CallContext, eventFn func(string, any)) {

	plan, planErr := s.planner.BuildPlan(callCtx.Query, decision.Intent, decision.Tools, callCtx)
	if planErr != nil {
		eventFn("error", map[string]any{"message": planErr.Error()})
		return
	}
	eventFn("plan", plan)
	if agentplanner.NeedsConfirmation(plan) {
		eventFn("confirm.required", map[string]any{
			"message": "此操作需要确认", "risk": plan.Risk,
			"planId": plan.PlanID,
		})
		return
	}
	// 无风险的 execute 直接执行
	decision2 := agentllm.Decision{Intent: decision.Intent, Tools: decision.Tools}
	s.streamSearch(ctx, callCtx.Query, decision2, callCtx, eventFn)
}
