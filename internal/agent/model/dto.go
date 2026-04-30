package model

import "time"

// QueryRequest 是检索型 Agent 查询请求。
type QueryRequest struct {
	Query            string `json:"query"`
	WorkspaceID      string `json:"workspaceId,omitempty"`
	StorageSettingID string `json:"storageSettingId,omitempty"`
	TraceID          string `json:"traceId,omitempty"`
}

// QueryResponse 是检索型 Agent 响应。
type QueryResponse struct {
	TraceID     string        `json:"traceId"`
	Intent      string        `json:"intent"`
	Sources     []string      `json:"sources"`
	Items       []any         `json:"items"`
	Summary     string        `json:"summary"`
	ToolResults []ToolResult  `json:"toolResults"`
	Partial     bool          `json:"partial"`
	CreatedAt   time.Time     `json:"createdAt"`
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
	ID               uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	TraceID          string    `gorm:"column:trace_id;type:varchar(64);index"`
	UserID           string    `gorm:"column:user_id;type:varchar(128);index"`
	WorkspaceID      string    `gorm:"column:workspace_id;type:varchar(128);index"`
	StorageSettingID string    `gorm:"column:storage_setting_id;type:varchar(128);index"`
	QueryText        string    `gorm:"column:query_text;type:varchar(1024)"`
	Intent           string    `gorm:"column:intent;type:varchar(64)"`
	ToolName         string    `gorm:"column:tool_name;type:varchar(64);index"`
	Status           string    `gorm:"column:status;type:varchar(32);index"`
	ErrorMessage     string    `gorm:"column:error_message;type:varchar(1024)"`
	LatencyMs        int64     `gorm:"column:latency_ms"`
	InputSnapshot    string    `gorm:"column:input_snapshot;type:text"`
	OutputSnapshot   string    `gorm:"column:output_snapshot;type:text"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (AuditLog) TableName() string { return "agent_audit_log" }
