package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	agentllm "myclouddrive-go/internal/agent/llm"
	agentmodel "myclouddrive-go/internal/agent/model"
	agenttool "myclouddrive-go/internal/agent/tool"
)

// searchMode 同步 search 模式：LLM 决策 → 工具执行 → 摘要 → 返回。
func (s *AgentService) searchMode(ctx context.Context, req agentmodel.QueryRequest,
	traceID, intent string, toolNames []string, scope string, callCtx agenttool.CallContext) (*agentmodel.QueryResponse, error) {

	resp := &agentmodel.QueryResponse{
		TraceID:     traceID,
		RouteMode:   "llm",
		Provider:    s.llm.Name(),
		Model:       s.llm.Model(),
		Intent:      intent,
		Sources:     []string{},
		Items:       []any{},
		Summary:     "",
		ToolResults: []agentmodel.ToolResult{},
		Partial:     false,
		CreatedAt:   time.Now(),
	}
	log.Printf("agent_step trace=%s step=query.start status=ok query=%q workspace=%s scope=%s mode=search", traceID, req.Query, callCtx.WorkspaceID, scope)
	resp = s.executeTools(ctx, traceID, toolNames, callCtx, resp)

	decision := agentllm.Decision{Intent: intent, Tools: toolNames}
	llmSummary, sumErr := s.llm.Summarize(ctx, req.Query, decision, resp.Items)
	if sumErr != nil {
		log.Printf("agent_step trace=%s step=llm.summarize status=error err=%q", traceID, sumErr.Error())
		llmSummary = ""
	} else {
		log.Printf("agent_step trace=%s step=llm.summarize status=ok", traceID)
	}
	resp.Summary = searchSummary(intent, len(resp.Items), resp.Partial, llmSummary)
	log.Printf("agent_step trace=%s step=query.end status=ok items=%d partial=%v", traceID, len(resp.Items), resp.Partial)
	return resp, nil
}

// streamSearch 流式 search 模式：逐步推送 LLM 决策、工具执行、摘要。
func (s *AgentService) streamSearch(ctx context.Context, query string, decision agentllm.Decision,
	callCtx agenttool.CallContext, eventFn func(string, any), state *agentmodel.StreamState) {

	items := make([]any, 0)
	sources := make([]string, 0)
	for _, toolName := range decision.Tools {
		eventFn("tool.start", map[string]any{"tool": toolName})
		if err := s.registry.MustAllowed(toolName); err != nil {
			eventFn("tool.error", map[string]any{"tool": toolName, "error": err.Error()})
			continue
		}
		started := time.Now()
		result, callErr := s.registry.Call(ctx, toolName, callCtx, 2*time.Second)
		latency := time.Since(started).Milliseconds()
		if callErr != nil {
			eventFn("tool.error", map[string]any{"tool": toolName, "error": callErr.Error(), "latencyMs": latency})
		} else {
			sources = append(sources, result.Source)
			items = append(items, result.Items...)
			state.ItemCount += len(result.Items)
			state.Dirty = true
			eventFn("tool.done", map[string]any{"tool": toolName, "source": result.Source, "count": len(result.Items), "latencyMs": latency, "items": result.Items})
		}
	}
	_ = sources

	eventFn("summarize.start", map[string]any{"items": len(items)})
	summaryBuf := ""
	sumErr := s.llm.SummarizeStream(ctx, query, decision, items, func(token string) {
		summaryBuf += token
		state.Summary = summaryBuf
		eventFn("summary.token", map[string]any{"token": token, "summary": summaryBuf})
	})
	if sumErr != nil {
		eventFn("summarize.error", map[string]any{"error": sumErr.Error()})
	} else {
		state.Summary = summaryBuf
		eventFn("summarize.done", map[string]any{"summary": summaryBuf})
	}
}

// executeTools 执行工具列表，收集结果。
func (s *AgentService) executeTools(ctx context.Context, traceID string, toolNames []string,
	callCtx agenttool.CallContext, resp *agentmodel.QueryResponse) *agentmodel.QueryResponse {

	for _, toolName := range toolNames {
		if err := s.registry.MustAllowed(toolName); err != nil {
			resp.Partial = true
			resp.ToolResults = append(resp.ToolResults, agentmodel.ToolResult{
				Tool: toolName, Status: "error", Message: err.Error(),
			})
			continue
		}
		started := time.Now()
		log.Printf("agent_step trace=%s step=tool.call.start tool=%s", traceID, toolName)
		result, callErr := s.registry.Call(ctx, toolName, callCtx, 2*time.Second)
		latency := time.Since(started).Milliseconds()
		tr := agentmodel.ToolResult{Tool: toolName, LatencyMs: latency}
		if callErr != nil {
			resp.Partial = true
			tr.Status = "error"
			tr.Message = callErr.Error()
		} else {
			tr.Status = "ok"
			resp.Sources = append(resp.Sources, result.Source)
			resp.Items = append(resp.Items, result.Items...)
		}
		log.Printf("agent_step trace=%s step=tool.call.end tool=%s status=%s latency_ms=%d", traceID, toolName, tr.Status, latency)
		resp.ToolResults = append(resp.ToolResults, tr)
	}
	return resp
}

func searchSummary(intent string, n int, partial bool, llmSummary string) string {
	s := fmt.Sprintf("intent=%s 命中 %d 条", intent, n)
	if partial {
		s += "（部分结果）"
	}
	if strings.TrimSpace(llmSummary) != "" {
		s += "；" + llmSummary
	}
	return s
}
