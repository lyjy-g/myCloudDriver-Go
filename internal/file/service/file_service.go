package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	filemodel "myclouddrive-go/internal/file/model"
	filedb "myclouddrive-go/internal/file/model/dbmodel"
	"myclouddrive-go/internal/framework/security"
	storagemodel "myclouddrive-go/internal/storage/model"
	storagesvc "myclouddrive-go/internal/storage/service"
)

// StorageGateway 定义文件模块依赖的最小存储能力集合。
type StorageGateway interface {
	Put(ctx context.Context, in storagemodel.ObjectPutInput) (storagemodel.ObjectInfo, error)
	PresignDownloadURL(ctx context.Context, key string, expire time.Duration) (string, error)
	Get(ctx context.Context, key string) (io.ReadCloser, storagemodel.ObjectInfo, error)
	Delete(ctx context.Context, key string) error
}

// FileService 是 file 模块的唯一实现。
type FileService struct {
	db      *gorm.DB
	mu      sync.RWMutex
	counter int64
	items   map[string]*filemodel.FileItem
	storage StorageGateway

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

// PrecheckUpload 上传预检（秒传判定 + 任务创建）。
func (s *FileService) PrecheckUpload(in filemodel.UploadInitInput, settingID string) (bool, string, error) {
	if strings.TrimSpace(in.FileName) == "" {
		return false, "", errors.New("fileName is required")
	}
	if in.FileSize <= 0 {
		return false, "", errors.New("fileSize must be positive")
	}
	if in.TotalParts <= 0 {
		return false, "", errors.New("totalParts must be positive")
	}
	parentID := strings.TrimSpace(in.ParentID)
	if parentID == "" {
		parentID = "root"
	}

	s.mu.RLock()
	for _, it := range s.items {
		if it.Deleted || it.IsDir {
			continue
		}
		if strings.TrimSpace(it.FileHash) != "" && strings.EqualFold(it.FileHash, strings.TrimSpace(in.FileHash)) {
			s.mu.RUnlock()
			return true, "", nil
		}
	}
	s.mu.RUnlock()

	taskID := s.initTransferTask(filemodel.UploadInitInput{
		FileName:    in.FileName,
		FileHash:    in.FileHash,
		FileSize:    in.FileSize,
		ContentType: in.ContentType,
		ParentID:    parentID,
		TotalParts:  in.TotalParts,
	}, settingID)
	return false, taskID, nil
}

// InitUpload 显式初始化上传任务。
func (s *FileService) InitUpload(in filemodel.UploadInitInput, settingID string) (string, error) {
	if strings.TrimSpace(in.FileName) == "" {
		return "", errors.New("fileName is required")
	}
	if in.FileSize <= 0 {
		return "", errors.New("fileSize must be positive")
	}
	if in.TotalParts <= 0 {
		return "", errors.New("totalParts must be positive")
	}
	return s.initTransferTask(in, settingID), nil
}

func (s *FileService) initTransferTask(in filemodel.UploadInitInput, settingID string) string {
	parentID := strings.TrimSpace(in.ParentID)
	if parentID == "" {
		parentID = "root"
	}
	now := time.Now()
	taskID := fmt.Sprintf("up_%d", now.UnixNano())
	task := &filemodel.TransferTask{
		TaskID:           taskID,
		StorageSettingID: strings.TrimSpace(settingID),
		FileName:         strings.TrimSpace(in.FileName),
		FileHash:         strings.TrimSpace(in.FileHash),
		FileSize:         in.FileSize,
		ContentType:      strings.TrimSpace(in.ContentType),
		ParentID:         parentID,
		TotalParts:       in.TotalParts,
		Status:           filemodel.TransferTaskUploading,
		CreatedAt:        now,
		UpdatedAt:        now,
		Chunks:           make(map[int][]byte),
	}

	s.transferMu.Lock()
	s.transferTasks[taskID] = task
	s.transferMu.Unlock()
	return taskID
}

// UploadChunk 上传单个分片。
func (s *FileService) UploadChunk(taskID string, partIndex int, chunk []byte, chunkHash string) error {
	if partIndex <= 0 {
		return errors.New("chunkIndex must be positive")
	}
	if len(chunk) == 0 {
		return errors.New("chunk is empty")
	}
	task := s.getTransferTask(taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	if task.Status == filemodel.TransferTaskPaused {
		return errors.New("transfer task is paused")
	}
	if task.Status == filemodel.TransferTaskCanceled {
		return errors.New("transfer task is canceled")
	}
	if task.Status == filemodel.TransferTaskCompleted {
		return nil
	}
	if partIndex > task.TotalParts {
		return errors.New("chunkIndex exceeds total parts")
	}
	if strings.TrimSpace(chunkHash) != "" {
		sum := sha256.Sum256(chunk)
		if !strings.EqualFold(chunkHash, hex.EncodeToString(sum[:])) {
			return errors.New("chunk hash mismatch")
		}
	}

	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	if _, ok := task.Chunks[partIndex]; !ok {
		task.UploadedPart++
		task.UploadedSize += int64(len(chunk))
	}
	task.Chunks[partIndex] = append([]byte(nil), chunk...)
	task.UpdatedAt = time.Now()
	return nil
}

// MergeUpload 合并分片并落到存储层。
func (s *FileService) MergeUpload(ctx context.Context, taskID string) (*filemodel.FileItem, error) {
	task := s.getTransferTask(taskID)
	if task == nil {
		return nil, errors.New("transfer task not found")
	}
	if task.Status == filemodel.TransferTaskCanceled {
		return nil, errors.New("transfer task is canceled")
	}
	if task.Status == filemodel.TransferTaskPaused {
		return nil, errors.New("transfer task is paused")
	}
	if len(task.Chunks) != task.TotalParts {
		return nil, fmt.Errorf("chunks incomplete: %d/%d", len(task.Chunks), task.TotalParts)
	}

	ordered := make([][]byte, 0, task.TotalParts)
	for i := 1; i <= task.TotalParts; i++ {
		chunk, ok := task.Chunks[i]
		if !ok {
			return nil, fmt.Errorf("missing chunk %d", i)
		}
		ordered = append(ordered, chunk)
	}
	merged := bytes.Join(ordered, nil)
	objectKey := fmt.Sprintf("uploads/%s/%s", time.Now().Format("20060102"), task.TaskID+"_"+task.FileName)
	size := int64(len(merged))
	if task.FileSize > 0 {
		size = task.FileSize
	}
	contentType := task.ContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	if s.storage != nil {
		if _, err := s.storage.Put(ctx, storagemodel.ObjectPutInput{
			Key:           objectKey,
			Reader:        bytes.NewReader(merged),
			ContentType:   contentType,
			ContentLength: &size,
			Metadata: map[string]string{
				"file_hash": task.FileHash,
				"task_id":   task.TaskID,
			},
		}); err != nil {
			return nil, err
		}
	}

	s.mu.Lock()
	now := time.Now()
	id := s.nextIDLocked()
	name := s.uniqueNameLocked(task.ParentID, task.FileName)
	item := &filemodel.FileItem{
		ID:               id,
		ParentID:         task.ParentID,
		StorageSettingID: strings.TrimSpace(task.StorageSettingID),
		Name:             name,
		IsDir:            false,
		Size:             size,
		FileHash:         task.FileHash,
		ObjectKey:        objectKey,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.items[id] = item
	s.mu.Unlock()

	s.transferMu.Lock()
	task.Status = filemodel.TransferTaskCompleted
	task.UpdatedAt = time.Now()
	delete(task.Chunks, 0)
	s.transferMu.Unlock()

	cp := *item
	if err := s.persistFileInfo(ctx, cp); err != nil {
		return nil, fmt.Errorf("persist merge result failed: %w", err)
	}
	return &cp, nil
}

// PauseTransfer 暂停任务。
func (s *FileService) PauseTransfer(taskID string) error {
	task := s.getTransferTask(taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	if task.Status == filemodel.TransferTaskCompleted || task.Status == filemodel.TransferTaskCanceled {
		return nil
	}
	s.transferMu.Lock()
	task.Status = filemodel.TransferTaskPaused
	task.UpdatedAt = time.Now()
	s.transferMu.Unlock()
	return nil
}

// ResumeTransfer 恢复任务。
func (s *FileService) ResumeTransfer(taskID string) error {
	task := s.getTransferTask(taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	if task.Status == filemodel.TransferTaskCompleted || task.Status == filemodel.TransferTaskCanceled {
		return nil
	}
	s.transferMu.Lock()
	task.Status = filemodel.TransferTaskUploading
	task.UpdatedAt = time.Now()
	s.transferMu.Unlock()
	return nil
}

// CancelTransfer 取消任务。
func (s *FileService) CancelTransfer(taskID string) error {
	task := s.getTransferTask(taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	s.transferMu.Lock()
	task.Status = filemodel.TransferTaskCanceled
	task.Chunks = map[int][]byte{}
	task.UpdatedAt = time.Now()
	s.transferMu.Unlock()
	return nil
}

// ListTransferTasks 返回传输任务快照。
func (s *FileService) ListTransferTasks() []filemodel.TransferTask {
	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	out := make([]filemodel.TransferTask, 0, len(s.transferTasks))
	for _, t := range s.transferTasks {
		cp := *t
		cp.Chunks = nil
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func (s *FileService) getTransferTask(taskID string) *filemodel.TransferTask {
	s.transferMu.Lock()
	defer s.transferMu.Unlock()
	return s.transferTasks[taskID]
}

// Ping 服务健康检查。
func (s *FileService) Ping(_ context.Context) (string, error) {
	return "file service ready", nil
}

// Home 返回首页信息。
func (s *FileService) Home(_ context.Context) filemodel.HomeInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var used int64
	recent := make([]filemodel.FileItem, 0)
	for _, it := range s.items {
		if it.Deleted {
			continue
		}
		if !it.IsDir {
			used += it.Size
		}
		recent = append(recent, *it)
	}
	sort.Slice(recent, func(i, j int) bool { return recent[i].UpdatedAt.After(recent[j].UpdatedAt) })
	if len(recent) > 10 {
		recent = recent[:10]
	}
	return filemodel.HomeInfo{UsedBytes: used, Recent: recent}
}

// List 返回文件列表。
func (s *FileService) List(ctx context.Context, parentID, keyword, settingID string) []filemodel.FileItem {
	if s.db != nil {
		items, err := s.listFromDB(ctx, parentID, keyword, settingID)
		if err == nil {
			return items
		}
		return []filemodel.FileItem{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]filemodel.FileItem, 0)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for _, it := range s.items {
		if it.Deleted {
			continue
		}
		if parentID != "" && it.ParentID != parentID {
			continue
		}
		if strings.TrimSpace(settingID) != "" && it.ParentID != "" && it.StorageSettingID != strings.TrimSpace(settingID) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(it.Name), keyword) {
			continue
		}
		result = append(result, *it)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
	return result
}

// ListDirs 返回目录列表。
func (s *FileService) ListDirs(ctx context.Context, parentID, settingID string) []filemodel.FileItem {
	items := s.List(ctx, parentID, "", settingID)
	result := make([]filemodel.FileItem, 0, len(items))
	for _, it := range items {
		if it.IsDir {
			result = append(result, it)
		}
	}
	return result
}

// Get 读取文件详情。
func (s *FileService) Get(ctx context.Context, fileID string) (*filemodel.FileItem, error) {
	if s.db != nil {
		if item, err := s.getFromDB(ctx, fileID); err == nil {
			return item, nil
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[fileID]
	if !ok || it.Deleted {
		return nil, errors.New("file not found")
	}
	cp := *it
	return &cp, nil
}

func (s *FileService) getFromDB(ctx context.Context, fileID string) (*filemodel.FileItem, error) {
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	var row filedb.FileInfo
	if err = s.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND workspace_id = ? AND is_deleted = 0", fileID, p.UserID, p.WorkspaceID).
		First(&row).Error; err != nil {
		return nil, err
	}
	item := &filemodel.FileItem{
		ID:               row.ID,
		ParentID:         normalizeParentOutput(row.ParentID),
		StorageSettingID: row.StoragePlatformSettingID,
		Name:             row.DisplayName,
		IsDir:            row.IsDir,
		Size:             row.Size,
		FileHash:         row.ContentMd5,
		ObjectKey:        row.ObjectKey,
		CreatedAt:        row.UploadTime,
		UpdatedAt:        row.UpdateTime,
		Deleted:          row.IsDeleted,
	}
	return item, nil
}

// CreateDirectory 创建目录（自动重名处理）。
func (s *FileService) CreateDirectory(ctx context.Context, parentID, name, settingID string) (*filemodel.FileItem, error) {
	if s.db != nil {
		return s.createDirectoryDB(ctx, parentID, name, settingID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if parentID == "" {
		parentID = "root"
	}
	if _, ok := s.items[parentID]; !ok {
		return nil, errors.New("parent not found")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "新建文件夹"
	}
	name = s.uniqueNameLocked(parentID, name)

	now := time.Now()
	id := s.nextIDLocked()
	it := &filemodel.FileItem{ID: id, ParentID: parentID, StorageSettingID: strings.TrimSpace(settingID), Name: name, IsDir: true, CreatedAt: now, UpdatedAt: now}
	s.items[id] = it
	cp := *it
	return &cp, nil
}

func (s *FileService) listFromDB(ctx context.Context, parentID, keyword, settingID string) ([]filemodel.FileItem, error) {
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	parent := normalizeParentID(parentID)

	query := s.db.WithContext(ctx).Model(&filedb.FileInfo{}).
		Where("user_id = ? AND workspace_id = ? AND is_deleted = 0", p.UserID, p.WorkspaceID)
	if parent == "" {
		query = query.Where("(parent_id = '' OR parent_id IS NULL OR parent_id = 'root' OR parent_id = 'ROOT')")
	} else {
		query = query.Where("parent_id = ?", parent)
	}
	if strings.TrimSpace(settingID) != "" {
		query = query.Where("storage_platform_setting_id = ?", strings.TrimSpace(settingID))
	}
	if strings.TrimSpace(keyword) != "" {
		kw := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("display_name LIKE ?", kw)
	}
	var rows []filedb.FileInfo
	if err = query.Order("is_dir desc, display_name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]filemodel.FileItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, filemodel.FileItem{
			ID:               row.ID,
			ParentID:         normalizeParentOutput(row.ParentID),
			StorageSettingID: row.StoragePlatformSettingID,
			Name:             row.DisplayName,
			IsDir:            row.IsDir,
			Size:             row.Size,
			FileHash:         row.ContentMd5,
			ObjectKey:        row.ObjectKey,
			CreatedAt:        row.UploadTime,
			UpdatedAt:        row.UpdateTime,
			Deleted:          row.IsDeleted,
		})
	}
	return items, nil
}

func (s *FileService) createDirectoryDB(ctx context.Context, parentID, name, settingID string) (*filemodel.FileItem, error) {
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	parent := normalizeParentID(parentID)
	name = strings.TrimSpace(name)
	if name == "" {
		name = "新建文件夹"
	}
	now := time.Now()
	id := fmt.Sprintf("dir_%d", now.UnixNano())
	insert := map[string]any{
		"id":                          id,
		"object_key":                  "",
		"original_name":               name,
		"display_name":                name,
		"suffix":                      "",
		"size":                        int64(0),
		"mime_type":                   "inode/directory",
		"is_dir":                      true,
		"parent_id":                   parent,
		"user_id":                     p.UserID,
		"workspace_id":                p.WorkspaceID,
		"content_md5":                 "",
		"storage_platform_setting_id": strings.TrimSpace(settingID),
		"upload_time":                 now,
		"update_time":                 now,
		"last_access_time":            now,
		"is_deleted":                  false,
		"deleted_time":                nil,
	}
	if err = s.db.WithContext(ctx).Table("file_info").Create(insert).Error; err != nil {
		return nil, err
	}
	return &filemodel.FileItem{
		ID:               id,
		ParentID:         normalizeParentOutput(parent),
		StorageSettingID: strings.TrimSpace(settingID),
		Name:             name,
		IsDir:            true,
		Size:             0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (s *FileService) persistFileInfo(ctx context.Context, item filemodel.FileItem) error {
	if s.db == nil {
		return nil
	}
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	fileHash := strings.TrimSpace(item.FileHash)
	if len(fileHash) > 32 {
		// content_md5 列长度为 32，统一按前 32 位存储，避免落库失败。
		fileHash = fileHash[:32]
	}
	insert := map[string]any{
		"id":                          item.ID,
		"object_key":                  item.ObjectKey,
		"original_name":               item.Name,
		"display_name":                item.Name,
		"suffix":                      fileSuffix(item.Name),
		"size":                        item.Size,
		"mime_type":                   "application/octet-stream",
		"is_dir":                      item.IsDir,
		"parent_id":                   normalizeParentID(item.ParentID),
		"user_id":                     p.UserID,
		"workspace_id":                p.WorkspaceID,
		"content_md5":                 fileHash,
		"storage_platform_setting_id": strings.TrimSpace(item.StorageSettingID),
		"upload_time":                 now,
		"update_time":                 now,
		"last_access_time":            now,
		"is_deleted":                  false,
		"deleted_time":                nil,
	}
	return s.db.WithContext(ctx).Table("file_info").Create(insert).Error
}

func normalizeParentID(parentID string) string {
	switch strings.TrimSpace(parentID) {
	case "", "root", "ROOT":
		return ""
	default:
		return strings.TrimSpace(parentID)
	}
}

func normalizeParentOutput(parentID string) string {
	if strings.TrimSpace(parentID) == "" {
		return "root"
	}
	return strings.TrimSpace(parentID)
}

func fileSuffix(name string) string {
	name = strings.TrimSpace(name)
	idx := strings.LastIndex(name, ".")
	if idx <= 0 || idx >= len(name)-1 {
		return ""
	}
	return strings.ToLower(name[idx+1:])
}

// Rename 重命名。
func (s *FileService) Rename(fileID, newName string) (*filemodel.FileItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	it, ok := s.items[fileID]
	if !ok || it.Deleted {
		return nil, errors.New("file not found")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, errors.New("name required")
	}
	it.Name = s.uniqueNameLocked(it.ParentID, newName)
	it.UpdatedAt = time.Now()
	cp := *it
	return &cp, nil
}

// Move 移动文件。
func (s *FileService) Move(fileIDs []string, targetParentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	parent, ok := s.items[targetParentID]
	if !ok || parent.Deleted || !parent.IsDir {
		return errors.New("target parent not found")
	}
	for _, id := range fileIDs {
		it, exists := s.items[id]
		if !exists || it.Deleted {
			continue
		}
		if it.IsDir && s.isDescendantLocked(targetParentID, it.ID) {
			return fmt.Errorf("cannot move dir into its child: %s", it.ID)
		}
		it.ParentID = targetParentID
		it.Name = s.uniqueNameLocked(targetParentID, it.Name)
		it.UpdatedAt = time.Now()
	}
	return nil
}

// Recycle 软删除。
func (s *FileService) Recycle(ctx context.Context, fileIDs []string) {
	if s.db != nil {
		p, err := security.RequireLogin(ctx)
		if err == nil && len(fileIDs) > 0 {
			now := time.Now()
			_ = s.db.WithContext(ctx).
				Model(&filedb.FileInfo{}).
				Where("id IN ? AND user_id = ? AND workspace_id = ? AND is_deleted = 0", fileIDs, p.UserID, p.WorkspaceID).
				Updates(map[string]any{
					"is_deleted":   true,
					"deleted_time": now,
					"update_time":  now,
				}).Error
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, id := range fileIDs {
		if it, ok := s.items[id]; ok && !it.Deleted {
			it.Deleted = true
			it.DeletedAt = &now
			it.UpdatedAt = now
		}
	}
}

// Restore 从回收站恢复。
func (s *FileService) Restore(ctx context.Context, fileIDs []string) {
	if s.db != nil {
		p, err := security.RequireLogin(ctx)
		if err == nil && len(fileIDs) > 0 {
			now := time.Now()
			_ = s.db.WithContext(ctx).
				Model(&filedb.FileInfo{}).
				Where("id IN ? AND user_id = ? AND workspace_id = ? AND is_deleted = 1", fileIDs, p.UserID, p.WorkspaceID).
				Updates(map[string]any{
					"is_deleted":   false,
					"deleted_time": nil,
					"update_time":  now,
				}).Error
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range fileIDs {
		if it, ok := s.items[id]; ok && it.Deleted {
			it.Deleted = false
			it.DeletedAt = nil
			it.UpdatedAt = time.Now()
			it.Name = s.uniqueNameLocked(it.ParentID, it.Name)
		}
	}
}

// PermanentlyDelete 永久删除（元数据 + 插件对象删除）。
//
// 说明：
// - 元数据删除先完成，保证用户视图一致性；
// - 对象删除逐个执行，失败写入报告，便于后续补偿任务清理。
func (s *FileService) PermanentlyDelete(ctx context.Context, fileIDs []string) filemodel.HardDeleteReport {
	report := filemodel.HardDeleteReport{
		Requested:        len(fileIDs),
		FailedObjectKeys: make([]string, 0),
	}

	s.mu.Lock()
	objectKeys := make([]string, 0)
	for _, id := range fileIDs {
		report.MetadataDeleted += s.collectDeleteTargetsLocked(id, &objectKeys)
		s.deleteRecursiveLocked(id)
	}
	s.mu.Unlock()

	if s.storage == nil || len(objectKeys) == 0 {
		return report
	}

	for _, key := range dedupeStrings(objectKeys) {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if err := s.storage.Delete(ctx, key); err != nil {
			report.ObjectDeleteFailed++
			report.FailedObjectKeys = append(report.FailedObjectKeys, key)
			continue
		}
		report.ObjectDeleteSuccess++
	}
	return report
}

// ClearRecycle 清空回收站（元数据 + 插件对象删除）。
func (s *FileService) ClearRecycle(ctx context.Context) filemodel.HardDeleteReport {
	report := filemodel.HardDeleteReport{FailedObjectKeys: make([]string, 0)}

	s.mu.Lock()
	objectKeys := make([]string, 0)
	for id, it := range s.items {
		if it.Deleted {
			report.Requested++
			report.MetadataDeleted += s.collectDeleteTargetsLocked(id, &objectKeys)
			delete(s.items, id)
		}
	}
	s.mu.Unlock()

	if s.storage == nil || len(objectKeys) == 0 {
		return report
	}
	for _, key := range dedupeStrings(objectKeys) {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if err := s.storage.Delete(ctx, key); err != nil {
			report.ObjectDeleteFailed++
			report.FailedObjectKeys = append(report.FailedObjectKeys, key)
			continue
		}
		report.ObjectDeleteSuccess++
	}
	return report
}

// ListRecycle 分页返回回收站。
func (s *FileService) ListRecycle(ctx context.Context, page, size int) ([]filemodel.FileItem, int) {
	if s.db != nil {
		p, err := security.RequireLogin(ctx)
		if err == nil {
			if page <= 0 {
				page = 1
			}
			if size <= 0 {
				size = 20
			}
			query := s.db.WithContext(ctx).Model(&filedb.FileInfo{}).
				Where("user_id = ? AND workspace_id = ? AND is_deleted = 1", p.UserID, p.WorkspaceID)
			var total int64
			if err = query.Count(&total).Error; err != nil {
				return []filemodel.FileItem{}, 0
			}
			var rows []filedb.FileInfo
			if err = query.Order("update_time desc").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
				return []filemodel.FileItem{}, int(total)
			}
			items := make([]filemodel.FileItem, 0, len(rows))
			for _, row := range rows {
				var deletedAt *time.Time
				if !row.DeletedTime.IsZero() {
					t := row.DeletedTime
					deletedAt = &t
				}
				items = append(items, filemodel.FileItem{
					ID:               row.ID,
					ParentID:         normalizeParentOutput(row.ParentID),
					StorageSettingID: row.StoragePlatformSettingID,
					Name:             row.DisplayName,
					IsDir:            row.IsDir,
					Size:             row.Size,
					FileHash:         row.ContentMd5,
					ObjectKey:        row.ObjectKey,
					CreatedAt:        row.UploadTime,
					UpdatedAt:        row.UpdateTime,
					Deleted:          row.IsDeleted,
					DeletedAt:        deletedAt,
				})
			}
			return items, int(total)
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	items := make([]filemodel.FileItem, 0)
	for _, it := range s.items {
		if it.Deleted {
			items = append(items, *it)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	total := len(items)
	start := (page - 1) * size
	if start >= total {
		return []filemodel.FileItem{}, total
	}
	end := start + size
	if end > total {
		end = total
	}
	return items[start:end], total
}

// SetFavorite 设置收藏状态。
func (s *FileService) SetFavorite(fileIDs []string, favorite bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, id := range fileIDs {
		if it, ok := s.items[id]; ok && !it.Deleted {
			it.Favorite = favorite
			it.UpdatedAt = now
		}
	}
}

// DirPath 返回目录层级路径。
func (s *FileService) DirPath(dirID string) ([]filemodel.FileItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cur, ok := s.items[dirID]
	if !ok || cur.Deleted || !cur.IsDir {
		return nil, errors.New("dir not found")
	}
	pathItems := make([]filemodel.FileItem, 0)
	for cur != nil {
		pathItems = append(pathItems, *cur)
		if cur.ParentID == "" {
			break
		}
		cur = s.items[cur.ParentID]
	}
	for i, j := 0, len(pathItems)-1; i < j; i, j = i+1, j-1 {
		pathItems[i], pathItems[j] = pathItems[j], pathItems[i]
	}
	return pathItems, nil
}

func (s *FileService) nextIDLocked() string {
	s.counter++
	return "f_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func (s *FileService) uniqueNameLocked(parentID, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unnamed"
	}
	base := name
	idx := 1
	for s.existsNameLocked(parentID, name) {
		name = fmt.Sprintf("%s(%d)", base, idx)
		idx++
	}
	return name
}

func (s *FileService) existsNameLocked(parentID, name string) bool {
	for _, it := range s.items {
		if it.Deleted {
			continue
		}
		if it.ParentID == parentID && it.Name == name {
			return true
		}
	}
	return false
}

func (s *FileService) isDescendantLocked(candidateID, ancestorID string) bool {
	if candidateID == ancestorID {
		return true
	}
	cur := s.items[candidateID]
	for cur != nil {
		if cur.ParentID == ancestorID {
			return true
		}
		if cur.ParentID == "" {
			return false
		}
		cur = s.items[cur.ParentID]
	}
	return false
}

func (s *FileService) deleteRecursiveLocked(id string) {
	for childID, it := range s.items {
		if it.ParentID == id {
			s.deleteRecursiveLocked(childID)
		}
	}
	delete(s.items, id)
}

func (s *FileService) collectDeleteTargetsLocked(id string, objectKeys *[]string) int {
	it, ok := s.items[id]
	if !ok {
		return 0
	}
	count := 1
	if !it.IsDir && strings.TrimSpace(it.ObjectKey) != "" {
		*objectKeys = append(*objectKeys, it.ObjectKey)
	}
	for childID, child := range s.items {
		if child.ParentID == id {
			count += s.collectDeleteTargetsLocked(childID, objectKeys)
		}
	}
	return count
}

func dedupeStrings(items []string) []string {
	if len(items) <= 1 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

// ResolveDownloadURL 通过统一存储门面生成下载 URL，业务层不关心 local/s3 细节。
func (s *FileService) ResolveDownloadURL(ctx context.Context, fileID string, expire time.Duration) (string, *filemodel.FileItem, error) {
	item, err := s.Get(ctx, fileID)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(item.ObjectKey) == "" || s.storage == nil {
		return fmt.Sprintf("/api/file/stream/preview/%s", fileID), item, nil
	}

	url, err := s.storage.PresignDownloadURL(ctx, item.ObjectKey, expire)
	if err != nil {
		// 插件不支持预签名或配置不可用时，降级为统一预览路由。
		return fmt.Sprintf("/api/file/stream/preview/%s", fileID), item, nil
	}
	return url, item, nil
}

// OpenPreviewContent 通过统一存储门面读取对象内容。
func (s *FileService) OpenPreviewContent(ctx context.Context, fileID string) (io.ReadCloser, storagemodel.ObjectInfo, *filemodel.FileItem, error) {
	item, err := s.Get(ctx, fileID)
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, nil, err
	}
	if strings.TrimSpace(item.ObjectKey) == "" || s.storage == nil {
		return nil, storagemodel.ObjectInfo{}, item, nil
	}

	rc, info, err := s.storage.Get(ctx, item.ObjectKey)
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, item, err
	}
	return rc, info, item, nil
}

// ExecuteIdempotent 在 file service 内执行进程内幂等控制，不依赖外部幂等表/独立服务。
func (s *FileService) ExecuteIdempotent(endpoint, idemKey string, requestBody []byte, execute func() (int, any, error)) (statusCode int, response any, replayed bool, err error) {
	idemKey = strings.TrimSpace(idemKey)
	if idemKey == "" {
		statusCode, response, err = execute()
		return statusCode, response, false, err
	}

	reqHash := hashRequest(endpoint, requestBody)
	recordKey := endpoint + "|" + idemKey
	now := time.Now()

	s.idemMu.Lock()
	if rec, ok := s.idemRecords[recordKey]; ok {
		if now.After(rec.ExpireAt) {
			delete(s.idemRecords, recordKey)
		} else {
			if rec.RequestHash != reqHash {
				s.idemMu.Unlock()
				return 0, nil, false, ErrIdempotencyConflict
			}
			if rec.Processing {
				s.idemMu.Unlock()
				return 0, nil, false, ErrIdempotencyInProgress
			}
			var replay any
			if len(rec.ResponseRaw) > 0 {
				if unmarshalErr := json.Unmarshal(rec.ResponseRaw, &replay); unmarshalErr != nil {
					replay = map[string]any{"raw": string(rec.ResponseRaw)}
				}
			}
			s.idemMu.Unlock()
			return rec.StatusCode, replay, true, nil
		}
	}

	s.idemRecords[recordKey] = idempotencyRecord{
		RequestHash: reqHash,
		Processing:  true,
		ExpireAt:    now.Add(s.idemTTL),
	}
	s.idemMu.Unlock()

	statusCode, response, err = execute()
	if err != nil {
		s.idemMu.Lock()
		delete(s.idemRecords, recordKey)
		s.idemMu.Unlock()
		return statusCode, response, false, err
	}

	raw, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		raw = []byte(`{"code":"OK","message":"success","data":{"marshal":"failed"}}`)
	}

	s.idemMu.Lock()
	s.idemRecords[recordKey] = idempotencyRecord{
		RequestHash: reqHash,
		Processing:  false,
		StatusCode:  statusCode,
		ResponseRaw: raw,
		ExpireAt:    time.Now().Add(s.idemTTL),
	}
	s.idemMu.Unlock()

	return statusCode, response, false, nil
}

func hashRequest(endpoint string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(endpoint))
	h.Write([]byte("|"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
