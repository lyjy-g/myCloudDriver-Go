package model

import "time"

// FileItem 表示文件元数据（当前为内存实现用 DTO）。
type FileItem struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	// StorageSettingID 仅用于服务端按激活配置隔离文件视图，前端无需感知。
	StorageSettingID string     `json:"-"`
	Name             string     `json:"name"`
	IsDir            bool       `json:"is_dir"`
	Size             int64      `json:"size"`
	FileHash         string     `json:"file_hash,omitempty"`
	ObjectKey        string     `json:"object_key,omitempty"`
	Favorite         bool       `json:"favorite"`
	Deleted          bool       `json:"deleted"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// HomeInfo 表示文件首页信息。
type HomeInfo struct {
	UsedBytes int64      `json:"used_bytes"`
	Recent    []FileItem `json:"recent"`
}

// HardDeleteReport 记录“元数据 + 对象存储”双写删除结果。
type HardDeleteReport struct {
	Requested           int      `json:"requested"`
	MetadataDeleted     int      `json:"metadataDeleted"`
	ObjectDeleteSuccess int      `json:"objectDeleteSuccess"`
	ObjectDeleteFailed  int      `json:"objectDeleteFailed"`
	FailedObjectKeys    []string `json:"failedObjectKeys,omitempty"`
}

// TransferTaskStatus 表示传输任务状态。
type TransferTaskStatus string

const (
	TransferTaskUploading TransferTaskStatus = "UPLOADING"
	TransferTaskPaused    TransferTaskStatus = "PAUSED"
	TransferTaskMerging   TransferTaskStatus = "MERGING"
	TransferTaskCompleted TransferTaskStatus = "COMPLETED"
	TransferTaskCanceled  TransferTaskStatus = "CANCELED"
)

// TransferTask 表示上传传输任务。
type TransferTask struct {
	TaskID           string             `json:"taskId"`
	UserID           string             `json:"-"`
	WorkspaceID      string             `json:"-"`
	StorageSettingID string             `json:"-"`
	ObjectKey        string             `json:"-"`
	FileName         string             `json:"fileName"`
	FileHash         string             `json:"fileHash"`
	FileSize         int64              `json:"fileSize"`
	ContentType      string             `json:"contentType"`
	ParentID         string             `json:"parentId"`
	TotalParts       int                `json:"totalParts"`
	UploadedSize     int64              `json:"uploadedSize"`
	UploadedPart     int                `json:"uploadedParts"`
	Status           TransferTaskStatus `json:"status"`
	Version          int                `json:"version"`
	CreatedAt        time.Time          `json:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt"`

	Chunks map[int][]byte `json:"-"`
}

// UploadInitInput 初始化上传入参。
type UploadInitInput struct {
	FileName    string
	FileHash    string
	FileSize    int64
	ContentType string
	ParentID    string
	TotalParts  int
}

// UploadPrecheckResult 表示上传预检结果。
type UploadPrecheckResult struct {
	SkipUpload     bool   `json:"skipUpload"`
	TaskID         string `json:"taskId,omitempty"`
	UploadID       string `json:"uploadId,omitempty"`
	WeakMatchCount int    `json:"weakMatchCount"`
	HashChecked    bool   `json:"hashChecked"`
	StrongMatch    bool   `json:"strongMatch"`
	// Progress 让“预检确认可上传”模式直接拿到任务初始快照，避免前端只能本地兜底构造进度。
	Progress    map[string]any `json:"progress,omitempty"`
	InstantFile *FileItem      `json:"instantFile,omitempty"`
}

// TransferCleanupJob 表示上传任务结束后的异步清理记录。
type TransferCleanupJob struct {
	JobID       string    `json:"jobId"`
	TaskID      string    `json:"taskId"`
	JobType     string    `json:"jobType"`
	Status      string    `json:"status"`
	RetryCount  int       `json:"retryCount"`
	ErrorMsg    string    `json:"errorMsg,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	NextRetryAt time.Time `json:"nextRetryAt,omitempty"`
	FinishedAt  time.Time `json:"finishedAt,omitempty"`
}
