package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	filemodel "myclouddrive-go/internal/file/model"
)

// PauseTransfer 暂停任务。
//
// 这里的处理顺序是：
// 1. 先读当前任务状态；
// 2. 遇到 MERGING/COMPLETED/CANCELED 直接拒绝；
// 3. 允许从 UPLOADING 切到 PAUSED；
// 4. 切换成功后同步刷新 DB 主状态和 Redis 热快照。
func (svc *FileService) PauseTransfer(ctx context.Context, taskID string) error {
	// 先拿当前任务快照，后续所有状态判断都基于这一份共享任务对象。
	task := svc.ensureTransferTask(ctx, taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	// 先挡住正在收尾的任务，避免用户看到“暂停成功”但任务其实马上就完成。
	if task.Status == filemodel.TransferTaskMerging {
		return errors.New("transfer task is merging, pause is not allowed")
	}
	if task.Status == filemodel.TransferTaskCompleted || task.Status == filemodel.TransferTaskCanceled {
		return nil
	}
	if !canTransitionTransferStatus(task.Status, filemodel.TransferTaskPaused) {
		return errors.New("transfer task status cannot pause")
	}
	// 这里先落 DB 再提交内存，保证“暂停成功”一定对应可恢复的主状态。
	return svc.transitionTransferTask(ctx, task, filemodel.TransferTaskPaused, nil)
}

// ResumeTransfer 恢复任务。
//
// 恢复和暂停一样，先看状态能不能回到 UPLOADING，再刷新 DB/Redis。
// 对已经完成或已取消的任务，直接保持终态，不做二次恢复。
func (svc *FileService) ResumeTransfer(ctx context.Context, taskID string) error {
	// 先拿当前任务快照，避免恢复时和并发取消/merge 各自用不同视图做判断。
	task := svc.ensureTransferTask(ctx, taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	// 已经进入合并阶段时，恢复没有意义，任务会很快结束。
	if task.Status == filemodel.TransferTaskCompleted || task.Status == filemodel.TransferTaskCanceled {
		return nil
	}
	// 恢复只能从暂停态回到上传态，其他跳跃式状态都不接受。
	if !canTransitionTransferStatus(task.Status, filemodel.TransferTaskUploading) {
		return errors.New("transfer task status cannot resume")
	}
	// 恢复也走同一套状态迁移 helper，避免和取消/merge 并发时互相覆盖。
	return svc.transitionTransferTask(ctx, task, filemodel.TransferTaskUploading, nil)
}

// CancelTransfer 取消任务。
//
// 取消不打断已经在飞的分片和 merge，但会阻断后续新请求。
// 这里把取消后的临时清理交给独立 job 记录，后面可以再接 MQ 或 worker 扫描。
func (svc *FileService) CancelTransfer(ctx context.Context, taskID string) error {
	// 取消同样先取共享任务对象，后续要基于它登记 cleanup job。
	task := svc.ensureTransferTask(ctx, taskID)
	if task == nil {
		return errors.New("transfer task not found")
	}
	if task.Status == filemodel.TransferTaskCompleted {
		return nil
	}
	// 合并中的任务已经在收尾，不打断，直接返回让它自己走完。
	if task.Status == filemodel.TransferTaskMerging {
		return errors.New("transfer task is merging, cancel is not allowed")
	}
	if !canTransitionTransferStatus(task.Status, filemodel.TransferTaskCanceled) {
		return errors.New("transfer task status cannot cancel")
	}
	// 取消先写主状态，成功后再登记清理任务，避免主状态没切成功却先发了清理。
	if err := svc.transitionTransferTask(ctx, task, filemodel.TransferTaskCanceled, nil); err != nil {
		return err
	}
	// 取消后只记录清理任务，不直接在主请求里删除临时分片。
	svc.registerCleanupJob(task.TaskID, "cancel_cleanup")
	return nil
}

// ListTransferTasks 返回传输任务快照。
//
// 这里只返回业务需要的热状态，不暴露底层 chunk 内容。
// 前端如果只想看进度，这里已经足够；如果要精确状态，再去单个任务详情接口。
func (svc *FileService) ListTransferTasks() []filemodel.TransferTask {
	// 这里加锁遍历任务表，避免返回过程中任务被并发修改。
	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	out := make([]filemodel.TransferTask, 0, len(svc.transferTasks))
	for _, t := range svc.transferTasks {
		// 返回给前端的是快照副本，不能把内部 chunk 映射直接暴露出去。
		cp := *t
		cp.Chunks = nil
		if snap := svc.loadTransferSnapshot(cp.TaskID); snap != nil {
			// 如果 Redis 里有更热的状态，优先用 Redis 覆盖展示字段。
			if v, ok := snap["status"].(string); ok && v != "" {
				cp.Status = filemodel.TransferTaskStatus(v)
			}
			if v, ok := snap["uploadedParts"].(float64); ok {
				cp.UploadedPart = int(v)
			}
			if v, ok := snap["uploadedSize"].(float64); ok {
				cp.UploadedSize = int64(v)
			}
		}
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

// getTransferTask 读取任务对象。
//
// 这里继续沿用锁保护，避免任务状态和分片集合在并发上传时被读到半更新数据。
func (svc *FileService) getTransferTask(taskID string) *filemodel.TransferTask {
	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	return svc.transferTasks[taskID]
}

// GetTransferTaskByID 返回任务快照，优先 Redis，便于前端低频轮询直接拿到真实进度。
//
// Redis 丢了就回退内存任务态，后续可以再按 DB 记录重建。
func (svc *FileService) GetTransferTaskByID(taskID string) (filemodel.TransferTask, bool) {
	// 查询详情时允许从 DB 把主任务恢复回内存，避免进程重启后前端轮询直接查不到任务。
	task := svc.ensureTransferTask(context.Background(), taskID)
	if task == nil {
		return filemodel.TransferTask{}, false
	}
	// 这里仍然走锁保护，避免单任务查询时读到半更新中的共享对象。
	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	task, ok := svc.transferTasks[taskID]
	if !ok {
		return filemodel.TransferTask{}, false
	}
	// 返回副本而不是原对象，避免上层误改共享状态。
	cp := *task
	cp.Chunks = nil
	if snap := svc.loadTransferSnapshot(cp.TaskID); snap != nil {
		// 详情查询和列表查询一样，优先展示 Redis 热状态。
		if v, ok := snap["status"].(string); ok && v != "" {
			cp.Status = filemodel.TransferTaskStatus(v)
		}
		if v, ok := snap["uploadedParts"].(float64); ok {
			cp.UploadedPart = int(v)
		}
		if v, ok := snap["uploadedSize"].(float64); ok {
			cp.UploadedSize = int64(v)
		}
	}
	return cp, true
}

// GetTransferTaskSnapshot 返回任务快照，供轮询和 SSE 进度展示使用。
//
// 这个方法只做展示层聚合，不做任何状态修改。
func (svc *FileService) GetTransferTaskSnapshot(taskID string) map[string]any {
	if task, ok := svc.GetTransferTaskByID(taskID); ok {
		return transferStatusProgress(&task)
	}
	return nil
}

// SubscribeTransferTask 让 handler 订阅单个任务的实时快照变化。
func (svc *FileService) SubscribeTransferTask(taskID string) chan map[string]any {
	// 这里把订阅入口显式暴露给 API 层，避免 handler 直接操作 service 内部 watcher 结构。
	return svc.subscribeTransferTask(taskID)
}

// UnsubscribeTransferTask 关闭单个任务 watcher，避免 SSE 连接断开后继续积压快照。
func (svc *FileService) UnsubscribeTransferTask(taskID string, ch chan map[string]any) {
	// 退订同样收口到 service，保证 watcher 生命周期只在 file service 内部管理。
	svc.unsubscribeTransferTask(taskID, ch)
}

// GetUploadedChunkIndexes 返回任务当前已落库/已缓存的分片索引列表。
func (svc *FileService) GetUploadedChunkIndexes(ctx context.Context, taskID string) []int {
	// 这个接口专门给断点续传和前端恢复用，统一按 service 内部口径返回升序分片列表。
	return svc.getUploadedChunkIndexes(ctx, taskID)
}

// persistTransferPartDB 把已上传分片写入 DB。
//
// 分片记录是恢复任务和重建 Redis 的依据，所以这里保持“分片一旦成功写入就入库”。
func (svc *FileService) persistTransferPartDB(ctx context.Context, task *filemodel.TransferTask, partIndex int, chunk []byte, objectKey string) error {
	if svc.db == nil {
		return nil
	}
	// 这里把分片主信息一次性组装好，确保 DB 恢复时能知道这片落在哪、大小多少、哈希多少。
	now := time.Now()
	insert := map[string]any{
		"task_id":    task.TaskID,
		"part_index": partIndex,
		"part_hash":  sha256Hex(chunk),
		"part_size":  len(chunk),
		"object_key": objectKey,
		"status":     "uploaded",
		"created_at": now,
		"updated_at": now,
	}
	return svc.db.WithContext(ctx).Table("file_transfer_part").Create(insert).Error
}

// transferSnapshot 把任务状态序列化成 JSON 字符串。
//
// 主要给 Redis 热快照使用，避免前端每次都去查 DB。
func (svc *FileService) transferSnapshot(task *filemodel.TransferTask) string {
	// 这里直接复用统一的进度结构，避免 Redis 快照和接口返回结构各自维护一份。
	payload, err := json.Marshal(transferStatusProgress(task))
	if err != nil {
		return ""
	}
	return string(payload)
}

// StartMergeTransfer 把任务切到 MERGING。
//
// 这个阶段的目标是明确告诉后续请求：任务已经进入收尾，不允许再暂停或取消。
func (svc *FileService) StartMergeTransfer(ctx context.Context, task *filemodel.TransferTask) error {
	if task == nil {
		return errors.New("transfer task not found")
	}
	// merge 一旦开始，就不能再回到暂停态或取消态。
	if !canTransitionTransferStatus(task.Status, filemodel.TransferTaskMerging) {
		return fmt.Errorf("transfer task status %s cannot enter merging", task.Status)
	}
	// merge 是保护态，只有 DB 真正切过去后才让后续请求看到 MERGING。
	return svc.transitionTransferTask(ctx, task, filemodel.TransferTaskMerging, nil)
}

// FinishMergeTransfer 把任务切到 COMPLETED，并登记清理任务。
//
// 合并成功后，任务就进入终态；临时分片的清理交给异步 job，避免阻塞主请求。
func (svc *FileService) FinishMergeTransfer(ctx context.Context, task *filemodel.TransferTask) error {
	if task == nil {
		return nil
	}
	// 合并完成后任务进入终态，同时把临时分片清理意图落成 job。
	if err := svc.transitionTransferTask(ctx, task, filemodel.TransferTaskCompleted, nil); err != nil {
		return err
	}
	svc.registerCleanupJob(task.TaskID, "merge_cleanup")
	return nil
}

// transitionTransferTask 统一处理任务状态迁移，保证 DB 成功后才提交内存与 Redis。
func (svc *FileService) transitionTransferTask(ctx context.Context, task *filemodel.TransferTask, target filemodel.TransferTaskStatus, mutator func(*filemodel.TransferTask)) error {
	if task == nil {
		return errors.New("transfer task not found")
	}
	// 这里先拷贝当前任务快照，避免 DB 更新失败时污染共享内存对象。
	svc.transferMu.Lock()
	candidate := *task
	// chunk map 只读复用，不在状态迁移里修改 chunk 内容，所以这里保留同一份引用。
	candidate.Chunks = task.Chunks
	svc.transferMu.Unlock()
	if !canTransitionTransferStatus(candidate.Status, target) {
		return fmt.Errorf("transfer task status %s cannot switch to %s", candidate.Status, target)
	}
	// 先在候选副本上改目标状态，只有 DB 提交成功后才会正式回写原对象。
	candidate.Status = target
	candidate.UpdatedAt = time.Now()
	if mutator != nil {
		// 上传进度、完成时间等伴随状态变化的字段都在这里一起改，避免分散到调用方遗漏。
		mutator(&candidate)
	}
	if err := svc.persistTransferTaskStatusWithVersion(ctx, &candidate, candidate.Version); err != nil {
		if errors.Is(err, ErrTransferTaskVersion) && svc.db != nil {
			// 乐观锁冲突时，把 DB 里的最新版本和状态回灌到内存，避免后续继续拿旧版本重试。
			row, loadErr := svc.loadTransferTaskRowState(ctx, candidate.TaskID)
			if loadErr == nil {
				svc.transferMu.Lock()
				// 这里把共享对象修正成 DB 最新状态，避免本进程里的后续请求继续拿旧 version。
				task.Status = filemodel.TransferTaskStatus(row.Status)
				task.Version = row.Version
				task.UploadedPart = row.UploadedParts
				task.UploadedSize = row.UploadedSize
				task.UpdatedAt = time.Now()
				svc.transferMu.Unlock()
				svc.persistTransferSnapshot(task)
			}
		}
		return err
	}
	svc.transferMu.Lock()
	// 走到这里说明 DB CAS 已成功，再把候选副本提交回共享内存对象。
	task.Status = candidate.Status
	task.UploadedPart = candidate.UploadedPart
	task.UploadedSize = candidate.UploadedSize
	task.Version = candidate.Version
	task.UpdatedAt = candidate.UpdatedAt
	svc.transferMu.Unlock()
	svc.persistTransferSnapshot(task)
	return nil
}
