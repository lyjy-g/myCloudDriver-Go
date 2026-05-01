package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	agentmodel "myclouddrive-go/internal/agent/model"
)

// Logger 负责 Agent 调用审计。
type Logger struct {
	db *gorm.DB
}

func NewLogger(db *gorm.DB) *Logger {
	return &Logger{db: db}
}

// EnsureSchema 启动时确保审计表存在。
func (l *Logger) EnsureSchema(ctx context.Context) error {
	if l == nil || l.db == nil {
		return nil
	}
	sql := `CREATE TABLE IF NOT EXISTS agent_audit_log (
id BIGINT PRIMARY KEY AUTO_INCREMENT,
trace_id VARCHAR(64) NOT NULL,
user_id VARCHAR(128) NOT NULL,
workspace_id VARCHAR(128) NOT NULL,
storage_setting_id VARCHAR(128) NOT NULL DEFAULT '',
route_mode VARCHAR(32) NOT NULL DEFAULT '',
llm_provider VARCHAR(64) NOT NULL DEFAULT '',
llm_model VARCHAR(128) NOT NULL DEFAULT '',
query_text VARCHAR(1024) NOT NULL,
intent VARCHAR(64) NOT NULL,
tool_name VARCHAR(64) NOT NULL,
error_category VARCHAR(32) NOT NULL DEFAULT '',
status VARCHAR(32) NOT NULL,
error_message VARCHAR(1024) NOT NULL DEFAULT '',
latency_ms BIGINT NOT NULL DEFAULT 0,
input_snapshot TEXT,
output_snapshot TEXT,
created_at DATETIME NOT NULL,
KEY idx_trace_id(trace_id),
KEY idx_workspace_tool(workspace_id,tool_name),
KEY idx_created_at(created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	if err := l.db.WithContext(ctx).Exec(sql).Error; err != nil {
		return err
	}
	// 老版本表结构平滑升级，避免线上已有表缺列导致审计信息丢失。
	patches := []string{
		"ALTER TABLE agent_audit_log ADD COLUMN route_mode VARCHAR(32) NOT NULL DEFAULT ''",
		"ALTER TABLE agent_audit_log ADD COLUMN llm_provider VARCHAR(64) NOT NULL DEFAULT ''",
		"ALTER TABLE agent_audit_log ADD COLUMN llm_model VARCHAR(128) NOT NULL DEFAULT ''",
		"ALTER TABLE agent_audit_log ADD COLUMN error_category VARCHAR(32) NOT NULL DEFAULT ''",
	}
	for _, p := range patches {
		_ = l.db.WithContext(ctx).Exec(p).Error
	}
	return nil
}

func clip(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) <= n {
		return v
	}
	return v[:n]
}

func toJSON(v any) string {
	bs, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("marshal_error:%v", err)
	}
	return string(bs)
}

// Write 写一条审计。
func (l *Logger) Write(ctx context.Context, row agentmodel.AuditLog) {
	if l == nil || l.db == nil {
		return
	}
	row.QueryText = clip(row.QueryText, 1024)
	row.ErrorMessage = clip(row.ErrorMessage, 1024)
	row.InputSnapshot = clip(row.InputSnapshot, 4096)
	row.OutputSnapshot = clip(row.OutputSnapshot, 4096)
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now()
	}
	_ = l.db.WithContext(ctx).Create(&row).Error
}

func ToInputSnapshot(v any) string  { return toJSON(v) }
func ToOutputSnapshot(v any) string { return toJSON(v) }
