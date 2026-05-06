package dbmodel

import "time"

const TableNameKnowledgeImportTask = "knowledge_import_task"

// KnowledgeImportTask 知识库导入异步任务。
type KnowledgeImportTask struct {
	ID               string    `gorm:"column:id;primaryKey;size:64" json:"id"`
	WorkspaceID      string    `gorm:"column:workspace_id;not null;size:128" json:"workspace_id"`
	KnowledgeBaseID  int64     `gorm:"column:knowledge_base_id;not null" json:"knowledge_base_id"`
	KnowledgeFileID  int64     `gorm:"column:knowledge_file_id;not null" json:"knowledge_file_id"`
	FileID           string    `gorm:"column:file_id;not null;size:128" json:"file_id"`
	StorageSettingID string    `gorm:"column:storage_setting_id;not null;size:128" json:"storage_setting_id"`
	Status           string    `gorm:"column:status;not null;size:32;default:pending" json:"status"` // pending/running/success/failed
	Stage            string    `gorm:"column:stage;not null;size:32;default:pending" json:"stage"`   // parsing/chunking/embedding/indexing
	Progress         int32     `gorm:"column:progress;not null;default:0" json:"progress"`
	ErrorCategory    string    `gorm:"column:error_category;size:32" json:"error_category"`
	ErrorMessage     string    `gorm:"column:error_message;type:text" json:"error_message"`
	RetryCount       int32     `gorm:"column:retry_count;not null;default:0" json:"retry_count"`
	CreatedAt        time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (*KnowledgeImportTask) TableName() string {
	return TableNameKnowledgeImportTask
}
