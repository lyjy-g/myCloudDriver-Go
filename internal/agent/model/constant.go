package model

// Mode 类型常量。
const (
	ModeSearch   = "search"
	ModeExecute  = "execute"
	ModeRAG      = "rag"
	ModeWorkflow = "workflow"
)

// RouteMode 路由模式。
const (
	RouteLLM        = "llm"
	RouteLLMExecute = "llm_execute"
	RouteDirect     = "direct"
)

// Run 状态常量。
const (
	RunStatusPending   = "pending"
	RunStatusRunning   = "running"
	RunStatusWaiting   = "waiting_confirm"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
)

// Step 状态常量。
const (
	StepStatusPending = "pending"
	StepStatusRunning = "running"
	StepStatusOK      = "ok"
	StepStatusError   = "error"
)

// Tool 类型常量。
const (
	ToolCategoryFile       = "file"
	ToolCategoryShare      = "share"
	ToolCategoryTransfer   = "transfer"
	ToolCategoryRAG        = "rag"
	ToolCategoryWorkflow   = "workflow"
)

// 预定义 tool 名称。
const (
	ToolFileList      = "tool.file.list"
	ToolFileSearch    = "tool.file.search"
	ToolFileStats     = "tool.file.stats"
	ToolFileTrashList = "tool.file.trash.list"
	ToolFileRank      = "tool.file.rank"
	ToolShareList     = "tool.share.list"
	ToolShareSearch   = "tool.share.search"
	ToolShareRecords  = "tool.share.records"
	ToolShareStats    = "tool.share.stats"
	ToolShareCreate   = "tool.share.create"
	ToolShareRevoke   = "tool.share.revoke"
	ToolRAGSearch     = "tool.rag.search"
	ToolWorkflow      = "tool.workflow"
)

// Scope 常量。
const (
	ScopeAuto           = "auto"
	ScopeWorkspace      = "workspace"
	ScopeStorageSetting = "storage_setting"
)
