package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"myclouddrive-go/internal/agent/workflow"
)

// registryAdapter 将 tool.Registry 适配为 workflow.ToolRunner。
type registryAdapter struct {
	reg *Registry
}

func (a *registryAdapter) MustAllowed(name string) error { return a.reg.MustAllowed(name) }
func (a *registryAdapter) Call(ctx context.Context, name string, callCtx workflow.ToolCallCtx, timeout time.Duration) (workflow.ToolCallResult, error) {
	result, err := a.reg.Call(ctx, name, CallContext{
		TraceID:          callCtx.TraceID,
		UserID:           callCtx.UserID,
		WorkspaceID:      callCtx.WorkspaceID,
		StorageSettingID: callCtx.StorageSettingID,
		Query:            callCtx.Query,
	}, timeout)
	return workflow.ToolCallResult{
		Source: result.Source,
		Items:  result.Items,
		Info:   result.Info,
	}, err
}

// WorkflowTool 工作流管理工具。
type WorkflowTool struct {
	engine *workflow.Engine
	defs   map[string]*workflow.Definition
}

func NewWorkflowTool(reg *Registry) *WorkflowTool {
	adapter := &registryAdapter{reg: reg}
	return &WorkflowTool{
		engine: workflow.NewEngine(adapter),
		defs:   make(map[string]*workflow.Definition),
	}
}

func (t *WorkflowTool) Name() string { return "tool.workflow" }

func (t *WorkflowTool) Register(def *workflow.Definition) {
	t.defs[def.ID] = def
}

func (t *WorkflowTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	q := strings.TrimSpace(callCtx.Query)
	var target *workflow.Definition
	for _, d := range t.defs {
		if strings.Contains(q, d.Name) || strings.Contains(q, d.ID) {
			target = d
			break
		}
	}
	if target == nil {
		wfIDs := make([]string, 0, len(t.defs))
		for id := range t.defs {
			wfIDs = append(wfIDs, id)
		}
		return ToolResult{
			Source: "workflow",
			Items: []any{map[string]any{
				"availableWorkflows": wfIDs,
				"message":            "使用指定 workflow ID 再次查询以启动编排执行",
			}},
			Info: fmt.Sprintf("available_workflows=%d", len(wfIDs)),
		}, nil
	}
	run, err := t.engine.Start(ctx, target, workflow.ToolCallCtx{
		TraceID:          callCtx.TraceID,
		UserID:           callCtx.UserID,
		WorkspaceID:      callCtx.WorkspaceID,
		StorageSettingID: callCtx.StorageSettingID,
		Query:            callCtx.Query,
	})
	if err != nil {
		return ToolResult{}, fmt.Errorf("workflow execution failed: %w", err)
	}
	items := []any{map[string]any{
		"runId":       run.ID,
		"workflowId":  run.WorkflowID,
		"status":      string(run.Status),
		"currentNode": run.CurrentNode,
		"nodeLogs":    run.NodeLogs,
		"error":       run.Error,
	}}
	return ToolResult{Source: "workflow", Items: items, Info: fmt.Sprintf("run=%s status=%s", run.ID, run.Status)}, nil
}
