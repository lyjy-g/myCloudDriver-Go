package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	agentdb "myclouddrive-go/internal/agent/model/dbmodel"

	"gorm.io/gorm"
)

// runService 管理 agent_action / agent_action_step / agent_tool_call 持久化。
type runService struct {
	db *gorm.DB
}

func newRunService(db *gorm.DB) *runService {
	return &runService{db: db}
}

// NewRunService 用于模块注入持久化能力。
func NewRunService(db *gorm.DB) *runService {
	return newRunService(db)
}

func (r *runService) createAction(ctx context.Context, runID, userID, workspaceID, query, mode, status, risk string) error {
	if r == nil || r.db == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	now := time.Now()
	row := &agentdb.AgentAction{
		ID:          runID,
		SessionID:   "",
		WorkspaceID: strings.TrimSpace(workspaceID),
		UserID:      strings.TrimSpace(userID),
		UserInput:   query,
		RunType:     mode,
		Status:      status,
		RiskLevel:   strings.ToLower(strings.TrimSpace(risk)),
		IsConfirm:   "",
		TraceID:     runID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *runService) updateActionStatus(ctx context.Context, runID, status, isConfirm string) error {
	if r == nil || r.db == nil || strings.TrimSpace(runID) == "" {
		return nil
	}
	updates := map[string]any{
		"status":     strings.TrimSpace(status),
		"updated_at": time.Now(),
	}
	if strings.TrimSpace(isConfirm) != "" {
		updates["is_confirm"] = strings.TrimSpace(isConfirm)
	}
	return r.db.WithContext(ctx).Model(&agentdb.AgentAction{}).Where("id = ?", runID).Updates(updates).Error
}

func (r *runService) nextStepNo(ctx context.Context, runID string) int32 {
	if r == nil || r.db == nil || strings.TrimSpace(runID) == "" {
		return 1
	}
	var maxNo int32
	_ = r.db.WithContext(ctx).Model(&agentdb.AgentActionStep{}).
		Where("run_id = ?", runID).
		Select("COALESCE(MAX(step_no), 0)").Scan(&maxNo).Error
	return maxNo + 1
}

func (r *runService) createStep(ctx context.Context, runID, stepType, content, status string) (int64, error) {
	if r == nil || r.db == nil || strings.TrimSpace(runID) == "" {
		return 0, nil
	}
	row := &agentdb.AgentActionStep{
		RunID:     runID,
		StepNo:    r.nextStepNo(ctx, runID),
		StepType:  stepType,
		Content:   content,
		Status:    status,
		CreatedAt: time.Now(),
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r *runService) upsertTool(ctx context.Context, toolName, risk string) error {
	if r == nil || r.db == nil || strings.TrimSpace(toolName) == "" {
		return nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&agentdb.AgentTool{}).Where("name = ?", toolName).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := time.Now()
	row := &agentdb.AgentTool{
		Name:        toolName,
		Description: toolName,
		SchemaJSON:  "{}",
		RiskLevel:   strings.ToLower(strings.TrimSpace(risk)),
		Enabled:     true,
		TimeoutMs:   2000,
		RetryTimes:  0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *runService) createToolCall(ctx context.Context, runID string, stepID int64, toolName string, input any, output any, status, errMsg string, costMs int64) error {
	if r == nil || r.db == nil || strings.TrimSpace(runID) == "" || strings.TrimSpace(toolName) == "" {
		return nil
	}
	inBytes, _ := json.Marshal(input)
	outBytes, _ := json.Marshal(output)
	row := &agentdb.AgentToolCall{
		RunID:          runID,
		StepID:         stepID,
		ToolName:       toolName,
		InputJSON:      string(inBytes),
		OutputJSON:     string(outBytes),
		Status:         status,
		ErrorMessage:   errMsg,
		IdempotencyKey: "",
		CostMs:         int32(costMs),
		CreatedAt:      time.Now(),
	}
	return r.db.WithContext(ctx).Create(row).Error
}

func classifyToolRiskForRecord(toolName string) string {
	switch toolName {
	case "tool.share.create":
		return "write"
	case "tool.share.revoke":
		return "danger"
	case "tool.file.rename", "tool.file.create_dir", "tool.file.move", "tool.file.restore", "tool.file.rebuild_index":
		return "write"
	case "tool.file.delete":
		return "danger"
	default:
		return "read"
	}
}

func routeModeToRunType(mode string) string {
	switch strings.TrimSpace(mode) {
	case "execute":
		return "execute"
	case "rag":
		return "rag"
	case "workflow":
		return "workflow"
	default:
		return "retrieve"
	}
}

func queryStatusToActionStatus(partial bool, err error) string {
	if err != nil {
		return "failed"
	}
	if partial {
		return "failed"
	}
	return "success"
}

func actionRiskFromMode(mode string) string {
	if strings.TrimSpace(mode) == "execute" {
		return "write"
	}
	return "read"
}

func (r *runService) noop() error {
	return nil
}
