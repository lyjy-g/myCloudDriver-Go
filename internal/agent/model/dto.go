package model

import "time"

// QueryRequest 是检索型 Agent 查询请求。
type QueryRequest struct {
	Query            string `json:"query"`
	KbID             string `json:"kbId,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Mode             string `json:"mode,omitempty"`
	WorkspaceID      string `json:"workspaceId,omitempty"`
	StorageSettingID string `json:"storageSettingId,omitempty"`
	TraceID          string `json:"traceId,omitempty"`
}

// QueryResponse 是检索型 Agent 响应。
type QueryResponse struct {
	TraceID     string       `json:"traceId"`
	RouteMode   string       `json:"routeMode"`
	Provider    string       `json:"provider,omitempty"`
	Model       string       `json:"model,omitempty"`
	Intent      string       `json:"intent"`
	Sources     []string     `json:"sources"`
	Items       []any        `json:"items"`
	Summary     string       `json:"summary"`
	ToolResults []ToolResult `json:"toolResults"`
	Partial     bool         `json:"partial"`
	CreatedAt   time.Time    `json:"createdAt"`
}

// ToolResult 记录单工具调用结果。
type ToolResult struct {
	Tool      string `json:"tool"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latencyMs"`
	Message   string `json:"message,omitempty"`
}

// AuditLog 是 agent 调用审计模型。
type AuditLog struct {
	ID               uint64    `json:"id"`
	TraceID          string    `json:"traceId"`
	UserID           string    `json:"userId"`
	WorkspaceID      string    `json:"workspaceId"`
	StorageSettingID string    `json:"storageSettingId"`
	RouteMode        string    `json:"routeMode"`
	LLMProvider      string    `json:"llmProvider"`
	LLMModel         string    `json:"llmModel"`
	QueryText        string    `json:"queryText"`
	Intent           string    `json:"intent"`
	ToolName         string    `json:"toolName"`
	ErrorCategory    string    `json:"errorCategory"`
	Status           string    `json:"status"`
	ErrorMessage     string    `json:"errorMessage"`
	LatencyMs        int64     `json:"latencyMs"`
	InputSnapshot    string    `json:"inputSnapshot"`
	OutputSnapshot   string    `json:"outputSnapshot"`
	CreatedAt        time.Time `json:"createdAt"`
}

// ShareStatEntry 分享统计条目。
type ShareStatEntry struct {
	ShareID   string `json:"shareId"`
	ShareName string `json:"shareName"`
	Value     int    `json:"value"`
}

// ExecuteRequest 执行型请求，包含 plan 确认信息。，包含 plan 确认信息。
type ExecuteRequest struct {
	TraceID        string `json:"traceId"`
	ConfirmedPlan  bool   `json:"confirmedPlan"`
	PlanID         string `json:"planId"`
	ConfirmedSteps []int  `json:"confirmedSteps"`
	OriginalQuery  string `json:"originalQuery"`
}

// RiskLevel 风险等级。
type RiskLevel string

const (
	RiskRead    RiskLevel = "READ"
	RiskWrite   RiskLevel = "WRITE"
	RiskDanger  RiskLevel = "DANGER"
	RiskExport  RiskLevel = "EXPORT"
	RiskCrossWS RiskLevel = "CROSS_WS"
)

// ExecutionPlan 执行计划。
type ExecutionPlan struct {
	PlanID  string          `json:"planId"`
	Steps   []ExecutionStep `json:"steps"`
	Summary string          `json:"summary"`
	Risk    RiskLevel       `json:"risk"`
}

// ExecutionStep 单步执行计划。
type ExecutionStep struct {
	Index       int       `json:"index"`
	Description string    `json:"description"`
	ToolName    string    `json:"toolName"`
	FileIDs     []string  `json:"fileIds"`
	Risk        RiskLevel `json:"risk"`
}

// Run 记录一次 agent 执行。
type Run struct {
	ID          string    `json:"id"`
	TraceID     string    `json:"traceId"`
	UserID      string    `json:"userId"`
	WorkspaceID string    `json:"workspaceId"`
	Mode        string    `json:"mode"`
	Query       string    `json:"query"`
	Intent      string    `json:"intent"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Step 记录执行中的单步。
type Step struct {
	ID        string    `json:"id"`
	RunID     string    `json:"runId"`
	Index     int       `json:"index"`
	ToolName  string    `json:"toolName"`
	Status    string    `json:"status"`
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	LatencyMs int64     `json:"latencyMs"`
	CreatedAt time.Time `json:"createdAt"`
}

// StreamState 流式查询的累计状态，用于停止时持久化部分结果。
type StreamState struct {
	TraceID     string
	UserID      string
	WorkspaceID string
	Query       string
	Mode        string
	Intent      string
	Summary     string
	ItemCount   int
	Dirty       bool // 是否有已产生的数据值得持久化
}
