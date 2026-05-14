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

	filemodel "myclouddrive-go/internal/file/model"
	storagemodel "myclouddrive-go/internal/storage/model"
)

// PrecheckUpload 上传预检（秒传判定 + 任务创建）。
func (svc *FileService) PrecheckUpload(in filemodel.UploadInitInput, settingID string) (bool, string, error) {
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

	svc.mu.RLock()
	for _, item := range svc.items {
		if item.Deleted || item.IsDir {
			continue
		}
		if strings.TrimSpace(item.FileHash) != "" && strings.EqualFold(item.FileHash, strings.TrimSpace(in.FileHash)) {
			svc.mu.RUnlock()
			return true, "", nil
		}
	}
	svc.mu.RUnlock()

	taskID := svc.initTransferTask(filemodel.UploadInitInput{
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
		sum := sha256.Sum256(chunk)
		if !strings.EqualFold(chunkHash, hex.EncodeToString(sum[:])) {
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
