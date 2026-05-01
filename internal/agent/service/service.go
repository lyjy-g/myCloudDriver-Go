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
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"

	"github.com/google/uuid"
)

// AgentService 是检索型 Agent 编排实现。
type AgentService struct {
	registry *agenttool.Registry
	llm      agentllm.Provider
}

func New(registry *agenttool.Registry, llm agentllm.Provider) *AgentService {
	return &AgentService{registry: registry, llm: llm}
}

func summary(intent string, n int, partial bool, llmSummary string) string {
	s := fmt.Sprintf("intent=%s 命中 %d 条", intent, n)
	if partial {
		s += "（部分结果）"
	}
	if strings.TrimSpace(llmSummary) != "" {
		s += "；" + llmSummary
	}
	return s
}

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
	if mode != "search" {
		return nil, code.New(code.BadRequest, "mode not implemented yet: "+mode)
	}
	if s.registry == nil {
		return nil, code.New(code.InternalError, "agent registry unavailable")
	}
	if s.llm == nil {
		return nil, code.New(code.InternalError, "agent llm unavailable")
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
	scope := strings.TrimSpace(req.Scope)
	if scope == "" || scope == "auto" {
		if storageSettingID != "" {
			scope = "storage_setting"
		} else {
			scope = "workspace"
		}
	}
	if scope == "workspace" {
		storageSettingID = ""
	}

	traceID := strings.TrimSpace(req.TraceID)
	if traceID == "" {
		traceID = "agt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
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

	callCtx := agenttool.CallContext{
		TraceID:          traceID,
		UserID:           strings.TrimSpace(principal.UserID),
		WorkspaceID:      workspaceID,
		StorageSettingID: storageSettingID,
		Query:            strings.TrimSpace(req.Query),
	}
	log.Printf("agent_step trace=%s step=query.start status=ok query=%q workspace=%s setting=%s scope=%s", traceID, req.Query, workspaceID, storageSettingID, scope)

	for _, toolName := range toolNames {
		if err = s.registry.MustAllowed(toolName); err != nil {
			return nil, code.New(code.NoPermission, err.Error())
		}
		started := time.Now()
		log.Printf("agent_step trace=%s step=tool.call.start tool=%s", traceID, toolName)
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
		log.Printf("agent_step trace=%s step=tool.call.end tool=%s status=%s latency_ms=%d err=%q", traceID, toolName, status, latency, errMsg)
		resp.ToolResults = append(resp.ToolResults, tr)
	}

	llmSummary, sumErr := s.llm.Summarize(ctx, req.Query, decision, resp.Items)
	if sumErr != nil {
		log.Printf("agent_step trace=%s step=llm.summarize status=error err=%q", traceID, sumErr.Error())
		llmSummary = ""
	} else {
		log.Printf("agent_step trace=%s step=llm.summarize status=ok", traceID)
	}
	resp.Summary = summary(intent, len(resp.Items), resp.Partial, llmSummary)
	log.Printf("agent_step trace=%s step=query.end status=ok items=%d partial=%v", traceID, len(resp.Items), resp.Partial)
	return resp, nil
}
