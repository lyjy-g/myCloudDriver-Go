package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	filemodel "myclouddrive-go/internal/file/model"
	filedb "myclouddrive-go/internal/file/model/dbmodel"
	"myclouddrive-go/internal/framework/security"
	storagemodel "myclouddrive-go/internal/storage/model"

	"gorm.io/gorm"
)

// PrecheckUpload 上传预检（秒传判定 + 任务创建）。
func (svc *FileService) PrecheckUpload(ctx context.Context, in filemodel.UploadInitInput, settingID string) (filemodel.UploadPrecheckResult, error) {
	if strings.TrimSpace(in.FileName) == "" {
		return filemodel.UploadPrecheckResult{}, errors.New("fileName is required")
	}
	if in.FileSize <= 0 {
		return filemodel.UploadPrecheckResult{}, errors.New("fileSize must be positive")
	}
	if in.TotalParts <= 0 {
		return filemodel.UploadPrecheckResult{}, errors.New("totalParts must be positive")
	}
	parentID := strings.TrimSpace(in.ParentID)
	if parentID == "" {
		parentID = "root"
	}
	in.ParentID = parentID

	if svc.db != nil {
		return svc.precheckUploadDB(ctx, in, strings.TrimSpace(settingID))
	}

	result := filemodel.UploadPrecheckResult{
		HashChecked: strings.TrimSpace(in.FileHash) != "",
	}
	// 弱预检只做候选筛选，避免每次都直接走整表 hash 判重。
	candidates := svc.findWeakCandidates(in.FileSize, fileSuffix(in.FileName), strings.TrimSpace(settingID))
	result.WeakMatchCount = len(candidates)
	if len(candidates) > 0 && result.HashChecked {
		for _, item := range candidates {
			if strings.EqualFold(strings.TrimSpace(item.FileHash), strings.TrimSpace(in.FileHash)) {
				// 强预检命中后直接复用已有对象内容，但仍要为当前目录创建一条新的文件记录。
				instantFile := svc.cloneInstantFile(item, in.ParentID, strings.TrimSpace(settingID))
				result.SkipUpload = true
				result.StrongMatch = true
				result.InstantFile = instantFile
				return result, nil
			}
		}
	}

	taskID := svc.initTransferTask(filemodel.UploadInitInput{
		FileName:    in.FileName,
		FileHash:    in.FileHash,
		FileSize:    in.FileSize,
		ContentType: in.ContentType,
		ParentID:    parentID,
		TotalParts:  in.TotalParts,
	}, settingID)
	result.TaskID = taskID
	result.UploadID = taskID
	return result, nil
}

// InitUpload 显式初始化上传任务。
func (svc *FileService) InitUpload(in filemodel.UploadInitInput, settingID string) (string, error) {
	if strings.TrimSpace(in.FileName) == "" {
		return "", errors.New("fileName is required")
	}
	if in.FileSize <= 0 {
		return "", errors.New("fileSize must be positive")
	}
	if in.TotalParts <= 0 {
		return "", errors.New("totalParts must be positive")
	}
	return svc.initTransferTask(in, settingID), nil
}

func (svc *FileService) initTransferTask(in filemodel.UploadInitInput, settingID string) string {
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

	svc.transferMu.Lock()
	svc.transferTasks[taskID] = task
	svc.transferMu.Unlock()
	return taskID
}

// UploadChunk 上传单个分片。
func (svc *FileService) UploadChunk(taskID string, partIndex int, chunk []byte, chunkHash string) error {
	if partIndex <= 0 {
		return errors.New("chunkIndex must be positive")
	}
	if len(chunk) == 0 {
		return errors.New("chunk is empty")
	}
	task := svc.getTransferTask(taskID)
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
		if !strings.EqualFold(chunkHash, sha256Hex(chunk)) {
			return errors.New("chunk hash mismatch")
		}
	}

	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	if _, ok := task.Chunks[partIndex]; !ok {
		task.UploadedPart++
		task.UploadedSize += int64(len(chunk))
	}
	task.Chunks[partIndex] = append([]byte(nil), chunk...)
	task.UpdatedAt = time.Now()
	return nil
}

// MergeUpload 合并分片并落到存储层。
func (svc *FileService) MergeUpload(ctx context.Context, taskID string) (*filemodel.FileItem, error) {
	task := svc.getTransferTask(taskID)
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
	mergedHash := sha256Hex(merged)
	// merge 后再次校验整文件 hash，防止前端分片乱序、内容损坏或错误复用 task。
	if strings.TrimSpace(task.FileHash) != "" && !strings.EqualFold(strings.TrimSpace(task.FileHash), mergedHash) {
		return nil, errors.New("file hash mismatch after merge")
	}
	if strings.TrimSpace(task.FileHash) == "" {
		task.FileHash = mergedHash
	}
	objectKey := fmt.Sprintf("uploads/%s/%s", time.Now().Format("20060102"), task.TaskID+"_"+task.FileName)
	size := int64(len(merged))
	if task.FileSize > 0 {
		size = task.FileSize
	}
	contentType := task.ContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	if svc.storage != nil {
		if _, err := svc.storage.Put(ctx, storagemodel.ObjectPutInput{
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

	svc.mu.Lock()
	now := time.Now()
	id := svc.nextIDLocked()
	name := svc.uniqueNameLocked(task.ParentID, task.FileName)
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
	svc.items[id] = item
	svc.mu.Unlock()

	svc.transferMu.Lock()
	task.Status = filemodel.TransferTaskCompleted
	task.UpdatedAt = time.Now()
	delete(task.Chunks, 0)
	svc.transferMu.Unlock()

	cp := *item
	if err := svc.persistFileInfo(ctx, cp); err != nil {
		return nil, fmt.Errorf("persist merge result failed: %w", err)
	}
	return &cp, nil
}

func (svc *FileService) findWeakCandidates(fileSize int64, suffix, settingID string) []*filemodel.FileItem {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	out := make([]*filemodel.FileItem, 0)
	for _, item := range svc.items {
		if item.Deleted || item.IsDir {
			continue
		}
		if strings.TrimSpace(settingID) != "" && item.StorageSettingID != strings.TrimSpace(settingID) {
			continue
		}
		if item.Size != fileSize {
			continue
		}
		if fileSuffix(item.Name) != suffix {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (svc *FileService) cloneInstantFile(src *filemodel.FileItem, parentID, settingID string) *filemodel.FileItem {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	now := time.Now()
	id := svc.nextIDLocked()
	name := svc.uniqueNameLocked(parentID, src.Name)
	item := &filemodel.FileItem{
		ID:               id,
		ParentID:         parentID,
		StorageSettingID: strings.TrimSpace(settingID),
		Name:             name,
		IsDir:            false,
		Size:             src.Size,
		FileHash:         src.FileHash,
		ObjectKey:        src.ObjectKey,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	svc.items[id] = item
	cp := *item
	return &cp
}

func (svc *FileService) precheckUploadDB(ctx context.Context, in filemodel.UploadInitInput, settingID string) (filemodel.UploadPrecheckResult, error) {
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return filemodel.UploadPrecheckResult{}, err
	}
	result := filemodel.UploadPrecheckResult{
		HashChecked: strings.TrimSpace(in.FileHash) != "",
	}
	suffix := fileSuffix(in.FileName)

	weakQuery := svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
		Where("user_id = ? AND workspace_id = ? AND is_deleted = 0 AND is_dir = 0 AND size = ? AND suffix = ?", p.UserID, p.WorkspaceID, in.FileSize, suffix)
	if settingID != "" {
		weakQuery = weakQuery.Where("storage_platform_setting_id = ?", settingID)
	}
	var weakMatchCount int64
	if err = weakQuery.Count(&weakMatchCount).Error; err != nil {
		return filemodel.UploadPrecheckResult{}, err
	}
	result.WeakMatchCount = int(weakMatchCount)

	if result.HashChecked {
		var source filedb.FileInfo
		// 强预检只在弱预检范围内再按 SHA-256 精确命中，避免 size/ext 相同导致误判秒传。
		strongQuery := svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
			Where("user_id = ? AND workspace_id = ? AND is_deleted = 0 AND is_dir = 0 AND size = ? AND suffix = ? AND content_sha256 = ?", p.UserID, p.WorkspaceID, in.FileSize, suffix, strings.TrimSpace(in.FileHash))
		if settingID != "" {
			strongQuery = strongQuery.Where("storage_platform_setting_id = ?", settingID)
		}
		err = strongQuery.Order("upload_time desc").Take(&source).Error
		if err == nil {
			instantFile, cloneErr := svc.cloneInstantFileDB(ctx, p.UserID, p.WorkspaceID, &source, in.ParentID, settingID)
			if cloneErr != nil {
				return filemodel.UploadPrecheckResult{}, cloneErr
			}
			result.SkipUpload = true
			result.StrongMatch = true
			result.InstantFile = instantFile
			return result, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return filemodel.UploadPrecheckResult{}, err
		}
	}

	taskID := svc.initTransferTask(in, settingID)
	result.TaskID = taskID
	result.UploadID = taskID
	return result, nil
}

func (svc *FileService) cloneInstantFileDB(ctx context.Context, userID, workspaceID string, src *filedb.FileInfo, parentID, settingID string) (*filemodel.FileItem, error) {
	name, err := svc.uniqueNameDB(ctx, userID, workspaceID, parentID, src.DisplayName)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	id := "f_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	parent := normalizeParentID(parentID)
	// 秒传只复制元数据并复用 object_key，不重新写底层对象内容。
	insert := map[string]any{
		"id":                          id,
		"object_key":                  src.ObjectKey,
		"original_name":               name,
		"display_name":                name,
		"suffix":                      src.Suffix,
		"size":                        src.Size,
		"mime_type":                   src.MimeType,
		"is_dir":                      false,
		"parent_id":                   parent,
		"user_id":                     userID,
		"workspace_id":                workspaceID,
		"content_sha256":              src.ContentSha256,
		"storage_platform_setting_id": strings.TrimSpace(settingID),
		"upload_time":                 now,
		"update_time":                 now,
		"last_access_time":            now,
		"is_deleted":                  false,
		"deleted_time":                nil,
	}
	if err = svc.db.WithContext(ctx).Table("file_info").Create(insert).Error; err != nil {
		return nil, err
	}
	return &filemodel.FileItem{
		ID:               id,
		ParentID:         normalizeParentOutput(parent),
		StorageSettingID: strings.TrimSpace(settingID),
		Name:             name,
		IsDir:            false,
		Size:             src.Size,
		FileHash:         strings.TrimSpace(src.ContentSha256),
		ObjectKey:        src.ObjectKey,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (svc *FileService) uniqueNameDB(ctx context.Context, userID, workspaceID, parentID, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unnamed"
	}
	parent := normalizeParentID(parentID)
	base := name
	current := name
	idx := 1
	for {
		var count int64
		if err := svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
			Where("user_id = ? AND workspace_id = ? AND is_deleted = 0 AND display_name = ?", userID, workspaceID, current).
			Where("(parent_id = ? OR (? = '' AND (parent_id = '' OR parent_id IS NULL OR parent_id = 'root' OR parent_id = 'ROOT')))", parent, parent).
			Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return current, nil
		}
		current = fmt.Sprintf("%s(%d)", base, idx)
		idx++
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
