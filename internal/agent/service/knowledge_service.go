package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	agentdb "myclouddrive-go/internal/agent/model/dbmodel"
	"myclouddrive-go/internal/framework/code"
)

// KnowledgeItem 是知识库列表项。
type KnowledgeItem struct {
	ID          int64     `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ListKnowledgeByWorkspace 返回当前空间下的知识库列表。
func (s *AgentService) ListKnowledgeByWorkspace(ctx context.Context, workspaceID string) ([]KnowledgeItem, error) {
	if s == nil || s.runSvc == nil || s.runSvc.db == nil {
		return nil, code.New(code.InternalError, "agent db unavailable")
	}
	ws := strings.TrimSpace(workspaceID)
	if ws == "" {
		return nil, code.New(code.BadRequest, "workspace required")
	}

	var rows []agentdb.Knowledge
	if err := s.runSvc.db.WithContext(ctx).
		Where("workspace_id = ?", ws).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, code.New(code.InternalError, "list knowledge failed: "+err.Error())
	}

	items := make([]KnowledgeItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, KnowledgeItem{
			ID:          row.ID,
			WorkspaceID: row.WorkspaceID,
			Name:        row.Name,
			Description: row.Description,
			CreatedBy:   row.CreatedBy,
			CreatedAt:   row.CreatedAt,
		})
	}
	return items, nil
}

// CreateKnowledge 在 workspace 下创建知识库。
func (s *AgentService) CreateKnowledge(ctx context.Context, workspaceID, userID, name, description string) (*KnowledgeItem, error) {
	if s == nil || s.runSvc == nil || s.runSvc.db == nil {
		return nil, code.New(code.InternalError, "agent db unavailable")
	}
	ws := strings.TrimSpace(workspaceID)
	uid := strings.TrimSpace(userID)
	kbName := strings.TrimSpace(name)
	if ws == "" {
		return nil, code.New(code.BadRequest, "workspace required")
	}
	if uid == "" {
		return nil, code.New(code.BadRequest, "user required")
	}
	if kbName == "" {
		return nil, code.New(code.BadRequest, "knowledge name is required")
	}

	var exists int64
	if err := s.runSvc.db.WithContext(ctx).
		Model(&agentdb.Knowledge{}).
		Where("workspace_id = ? AND name = ?", ws, kbName).
		Count(&exists).Error; err != nil {
		return nil, code.New(code.InternalError, "check knowledge duplicate failed: "+err.Error())
	}
	if exists > 0 {
		return nil, code.New(code.BadRequest, "knowledge name already exists")
	}

	row := &agentdb.Knowledge{
		WorkspaceID: ws,
		Name:        kbName,
		Description: strings.TrimSpace(description),
		CreatedBy:   uid,
		CreatedAt:   time.Now(),
	}
	if err := s.runSvc.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, code.New(code.InternalError, "create knowledge failed: "+err.Error())
	}
	return &KnowledgeItem{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Name:        row.Name,
		Description: row.Description,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
	}, nil
}

// DeleteKnowledge 删除知识库及其文件/切片关联记录。
func (s *AgentService) DeleteKnowledge(ctx context.Context, workspaceID string, knowledgeID int64) error {
	if s == nil || s.runSvc == nil || s.runSvc.db == nil {
		return code.New(code.InternalError, "agent db unavailable")
	}
	ws := strings.TrimSpace(workspaceID)
	if ws == "" {
		return code.New(code.BadRequest, "workspace required")
	}
	if knowledgeID <= 0 {
		return code.New(code.BadRequest, "invalid knowledge id")
	}

	return s.runSvc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row agentdb.Knowledge
		if err := tx.
			Where("id = ? AND workspace_id = ?", knowledgeID, ws).
			First(&row).Error; err != nil {
			return code.New(code.NotFound, "knowledge not found")
		}

		if err := tx.
			Where("knowledge_base_id = ? AND workspace_id = ?", knowledgeID, ws).
			Delete(&agentdb.KnowledgeFile{}).Error; err != nil {
			return code.New(code.InternalError, "delete knowledge files failed: "+err.Error())
		}
		if err := tx.
			Where("knowledge_base_id = ?", knowledgeID).
			Delete(&agentdb.KnowledgeDocumentChunk{}).Error; err != nil {
			return code.New(code.InternalError, "delete knowledge chunks failed: "+err.Error())
		}
		if err := tx.
			Where("id = ? AND workspace_id = ?", knowledgeID, ws).
			Delete(&agentdb.Knowledge{}).Error; err != nil {
			return code.New(code.InternalError, "delete knowledge failed: "+err.Error())
		}
		return nil
	})
}
