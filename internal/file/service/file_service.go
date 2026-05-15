package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	filemodel "myclouddrive-go/internal/file/model"
	storagesvc "myclouddrive-go/internal/storage/service"
)

// FileService 是 file 模块的唯一实现。
type FileService struct {
	db      *gorm.DB
	mu      sync.RWMutex
	counter int64
	items   map[string]*filemodel.FileItem
	storage IStoragePower

	idemMu      sync.Mutex
	idemRecords map[string]idempotencyRecord
	idemTTL     time.Duration

	transferMu    sync.Mutex
	transferTasks map[string]*filemodel.TransferTask
}

type idempotencyRecord struct {
	RequestHash string
	Processing  bool
	StatusCode  int
	ResponseRaw []byte
	ExpireAt    time.Time
}

// 幂等冲突相关错误，用于 handler 层映射 HTTP 状态码。
var (
	ErrIdempotencyConflict   = errors.New("idempotency key payload conflict")
	ErrIdempotencyInProgress = errors.New("idempotency request in progress")
)

// NewFileService 创建文件服务。
func NewFileService(storage *storagesvc.StorageService, extras ...any) *FileService {
	var db *gorm.DB
	for _, extra := range extras {
		if v, ok := extra.(*gorm.DB); ok {
			db = v
			break
		}
	}
	now := time.Now()
	root := &filemodel.FileItem{
		ID:        "root",
		ParentID:  "",
		Name:      "/",
		IsDir:     true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return &FileService{
		db:            db,
		counter:       1,
		items:         map[string]*filemodel.FileItem{root.ID: root},
		storage:       storage,
		idemRecords:   make(map[string]idempotencyRecord),
		idemTTL:       24 * time.Hour,
		transferTasks: make(map[string]*filemodel.TransferTask),
	}
}

// Ping 服务健康检查。
func (svc *FileService) Ping(_ context.Context) (string, error) {
	return "file service ready", nil
}
