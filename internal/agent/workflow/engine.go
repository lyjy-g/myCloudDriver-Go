package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// ToolRunner 工作流引擎依赖的最小工具调用接口。
type ToolRunner interface {
	MustAllowed(name string) error
	Call(ctx context.Context, name string, callCtx ToolCallCtx, timeout time.Duration) (ToolCallResult, error)
}

// ToolCallCtx 工作流工具调用上下文。
type ToolCallCtx struct {
	TraceID          string
	UserID           string
	WorkspaceID      string
	StorageSettingID string
	Query            string
}

// ToolCallResult 工作流工具调用结果。
type ToolCallResult struct {
	Source string
	Items  []any
	Info   string
}

// RunStatus 运行状态。
type RunStatus string

const (
	StatusPending   RunStatus = "pending"
	StatusRunning   RunStatus = "running"
	StatusWaiting   RunStatus = "waiting_confirm"
	StatusCompleted RunStatus = "completed"
	StatusFailed    RunStatus = "failed"
)

// Run 一次工作流执行记录。
type Run struct {
	ID          string         `json:"id"`
	WorkflowID  string         `json:"workflowId"`
	Status      RunStatus      `json:"status"`
	CurrentNode string         `json:"currentNode"`
	Vars        map[string]any `json:"vars"`
	NodeLogs    []NodeLog      `json:"nodeLogs"`
	StartedAt   time.Time      `json:"startedAt"`
	EndedAt     *time.Time     `json:"endedAt,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// NodeLog 节点执行日志。
type NodeLog struct {
	NodeID    string    `json:"nodeId"`
	NodeType  NodeType  `json:"nodeType"`
	Status    string    `json:"status"`
	Output    string    `json:"output"`
	LatencyMs int64     `json:"latencyMs"`
	At        time.Time `json:"at"`
}

// Engine 工作流执行引擎。
type Engine struct {
	runner ToolRunner
}

func NewEngine(runner ToolRunner) *Engine {
	return &Engine{runner: runner}
}

// Start 启动工作流，返回 runID。执行到第一个 blocking 节点（如 manual_confirm）或结束。
func (e *Engine) Start(ctx context.Context, def *Definition, toolCtx ToolCallCtx) (*Run, error) {
	if err := def.Validate(); err != nil {
		return nil, err
	}
	run := &Run{
		ID:          fmt.Sprintf("run_%d", time.Now().UnixNano()),
		WorkflowID:  def.ID,
		Status:      StatusRunning,
		CurrentNode: def.StartNodeID,
		Vars:        copyVars(def.Vars),
		NodeLogs:    make([]NodeLog, 0),
		StartedAt:   time.Now(),
	}
	run, err := e.execute(ctx, def, run, toolCtx)
	if err != nil {
		run.Status = StatusFailed
		run.Error = err.Error()
		now := time.Now()
		run.EndedAt = &now
	}
	return run, err
}

// Resume 从 waiting_confirm 状态继续执行。
func (e *Engine) Resume(ctx context.Context, def *Definition, run *Run, confirmed bool, toolCtx ToolCallCtx) (*Run, error) {
	node := e.findNode(def, run.CurrentNode)
	if node == nil || node.Type != NodeManualConfirm {
		return nil, fmt.Errorf("cannot resume: current node is not manual_confirm")
	}
	if !confirmed {
		run.Status = StatusFailed
		run.Error = "user rejected confirmation"
		now := time.Now()
		run.EndedAt = &now
		return run, nil
	}
	run.Status = StatusRunning
	nextID := node.Next
	if nextID == "" {
		run.Status = StatusCompleted
		now := time.Now()
		run.EndedAt = &now
		return run, nil
	}
	run.CurrentNode = nextID
	var err error
	run, err = e.execute(ctx, def, run, toolCtx)
	if err != nil {
		run.Status = StatusFailed
		run.Error = err.Error()
		now := time.Now()
		run.EndedAt = &now
	}
	return run, err
}

func (e *Engine) execute(ctx context.Context, def *Definition, run *Run, toolCtx ToolCallCtx) (*Run, error) {
	for {
		node := e.findNode(def, run.CurrentNode)
		if node == nil {
			run.Status = StatusCompleted
			now := time.Now()
			run.EndedAt = &now
			return run, nil
		}
		started := time.Now()
		log.Printf("workflow_run %s step=%s type=%s", run.ID, node.ID, node.Type)
		switch node.Type {
		case NodeTool:
			result, err := e.executeToolNode(ctx, node, toolCtx)
			latency := time.Since(started).Milliseconds()
			run.NodeLogs = append(run.NodeLogs, NodeLog{
				NodeID: node.ID, NodeType: node.Type, LatencyMs: latency, At: time.Now(),
				Status: statusFrom(err), Output: truncate(fmt.Sprintf("%v", result), 200),
			})
			if err != nil {
				return run, err
			}
			run.Vars["_lastToolOutput"] = result
			run.Vars["items"] = result.Items
			if node.Next == "" {
				run.Status = StatusCompleted
				now := time.Now()
				run.EndedAt = &now
				return run, nil
			}
			run.CurrentNode = node.Next

		case NodeCondition:
			branch := e.evaluateCondition(node, run)
			latency := time.Since(started).Milliseconds()
			run.NodeLogs = append(run.NodeLogs, NodeLog{
				NodeID: node.ID, NodeType: node.Type, LatencyMs: latency, At: time.Now(),
				Status: "ok", Output: fmt.Sprintf("branch=%v", branch),
			})
			nextID := node.NextOnNO
			if branch {
				nextID = node.NextOnOK
			}
			if nextID == "" {
				run.Status = StatusCompleted
				now := time.Now()
				run.EndedAt = &now
				return run, nil
			}
			run.CurrentNode = nextID

		case NodeLLM:
			run.NodeLogs = append(run.NodeLogs, NodeLog{
				NodeID: node.ID, NodeType: node.Type, LatencyMs: time.Since(started).Milliseconds(), At: time.Now(),
				Status: "ok", Output: "llm node executed (placeholder)",
			})
			if node.Next == "" {
				run.Status = StatusCompleted
				now := time.Now()
				run.EndedAt = &now
				return run, nil
			}
			run.CurrentNode = node.Next

		case NodeManualConfirm:
			run.NodeLogs = append(run.NodeLogs, NodeLog{
				NodeID: node.ID, NodeType: node.Type, LatencyMs: time.Since(started).Milliseconds(), At: time.Now(),
				Status: "waiting",
			})
			run.Status = StatusWaiting
			return run, nil

		default:
			return run, fmt.Errorf("unknown node type: %s", node.Type)
		}
	}
}

func (e *Engine) executeToolNode(ctx context.Context, node *Node, toolCtx ToolCallCtx) (ToolCallResult, error) {
	var cfg ToolConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return ToolCallResult{}, fmt.Errorf("parse tool config: %w", err)
	}
	return e.runner.Call(ctx, cfg.ToolName, toolCtx, 5*time.Second)
}

func (e *Engine) evaluateCondition(node *Node, run *Run) bool {
	var cfg ConditionConfig
	if err := json.Unmarshal(node.Config, &cfg); err != nil {
		return false
	}
	val := resolveVar(run.Vars, cfg.Field)
	switch cfg.Operator {
	case "gt":
		v1, _ := strconv.ParseFloat(fmt.Sprintf("%v", val), 64)
		v2, _ := strconv.ParseFloat(cfg.Value, 64)
		return v1 > v2
	case "eq":
		return fmt.Sprintf("%v", val) == cfg.Value
	case "contains":
		return strings.Contains(strings.ToLower(fmt.Sprintf("%v", val)), strings.ToLower(cfg.Value))
	case "empty":
		return val == nil || fmt.Sprintf("%v", val) == "" || fmt.Sprintf("%v", val) == "0"
	}
	return false
}

func (e *Engine) findNode(def *Definition, id string) *Node {
	for i := range def.Nodes {
		if def.Nodes[i].ID == id {
			return &def.Nodes[i]
		}
	}
	return nil
}

func resolveVar(vars map[string]any, path string) any {
	parts := strings.Split(path, ".")
	current := any(vars)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return current
		}
		current = m[part]
	}
	return current
}

func statusFrom(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func copyVars(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
