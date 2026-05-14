package service

import (
	"errors"
	"sort"
	"time"

	filemodel "myclouddrive-go/internal/file/model"
)

// PauseTransfer 暂停任务。
func (svc *FileService) PauseTransfer(taskID string) error {
	task := svc.getTransferTask(taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	if task.Status == filemodel.TransferTaskCompleted || task.Status == filemodel.TransferTaskCanceled {
		return nil
	}
	svc.transferMu.Lock()
	task.Status = filemodel.TransferTaskPaused
	task.UpdatedAt = time.Now()
	svc.transferMu.Unlock()
	return nil
}

// ResumeTransfer 恢复任务。
func (svc *FileService) ResumeTransfer(taskID string) error {
	task := svc.getTransferTask(taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	if task.Status == filemodel.TransferTaskCompleted || task.Status == filemodel.TransferTaskCanceled {
		return nil
	}
	svc.transferMu.Lock()
	task.Status = filemodel.TransferTaskUploading
	task.UpdatedAt = time.Now()
	svc.transferMu.Unlock()
	return nil
}

// CancelTransfer 取消任务。
func (svc *FileService) CancelTransfer(taskID string) error {
	task := svc.getTransferTask(taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	svc.transferMu.Lock()
	task.Status = filemodel.TransferTaskCanceled
	task.Chunks = map[int][]byte{}
	task.UpdatedAt = time.Now()
	svc.transferMu.Unlock()
	return nil
}

// ListTransferTasks 返回传输任务快照。
func (svc *FileService) ListTransferTasks() []filemodel.TransferTask {
	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	out := make([]filemodel.TransferTask, 0, len(svc.transferTasks))
	for _, t := range svc.transferTasks {
		cp := *t
		cp.Chunks = nil
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

func (svc *FileService) getTransferTask(taskID string) *filemodel.TransferTask {
	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	return svc.transferTasks[taskID]
}
