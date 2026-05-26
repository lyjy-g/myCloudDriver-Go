package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	// 预检阶段先收紧必填参数，避免后面又走秒传又建任务后才发现基础参数不合法。
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
		// 没传父目录时统一落到 root，避免后面同一逻辑反复判空 parent。
		parentID = "root"
	}
	in.ParentID = parentID

	// 有 DB 就走 DB 秒传链路，没有 DB 就退回内存态，避免把两种存储都写成两套逻辑。
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
			// 强预检只有 hash 完全一致才秒传，避免 size/ext 一样就误判。
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

	taskID := svc.initTransferTask(ctx, filemodel.UploadInitInput{
		FileName:    in.FileName,
		FileHash:    in.FileHash,
		FileSize:    in.FileSize,
		ContentType: in.ContentType,
		ParentID:    parentID,
		TotalParts:  in.TotalParts,
	}, settingID)
	// 没命中秒传时，预检阶段只返回任务 id，不在这里预先占用任何分片状态。
	result.TaskID = taskID
	result.UploadID = taskID
	// 这里直接返回任务初始快照，补齐 precheck-progress 模式的后端真实语义。
	result.Progress = svc.GetTransferTaskSnapshot(taskID)
	return result, nil
}

// InitUpload 显式初始化上传任务。
func (svc *FileService) InitUpload(ctx context.Context, in filemodel.UploadInitInput, settingID string) (string, error) {
	// 显式建任务和预检一样，先把基础参数挡住，避免脏任务落到主表。
	if strings.TrimSpace(in.FileName) == "" {
		return "", errors.New("fileName is required")
	}
	if in.FileSize <= 0 {
		return "", errors.New("fileSize must be positive")
	}
	if in.TotalParts <= 0 {
		return "", errors.New("totalParts must be positive")
	}
	// 显式初始化就是只创建任务，不做秒传判断。
	return svc.initTransferTask(ctx, in, settingID), nil
}

// initTransferTask 创建一条新的上传任务，并同步初始化内存、DB、Redis 三处状态。
func (svc *FileService) initTransferTask(ctx context.Context, in filemodel.UploadInitInput, settingID string) string {
	parentID := strings.TrimSpace(in.ParentID)
	if parentID == "" {
		// 上传任务内部统一用 root 作为缺省父目录，减少后续状态迁移时的额外判断。
		parentID = "root"
	}
	now := time.Now()
	taskID := fmt.Sprintf("up_%d", now.UnixNano())
	objectKey := fmt.Sprintf("uploads/%s/%s", now.Format("20060102"), taskID+"_"+strings.TrimSpace(in.FileName))
	// 这里尽量把任务主数据一次性准备完整，后面分片上传只改状态和已上传信息。
	principal, _ := security.RequireLogin(ctx)
	task := &filemodel.TransferTask{
		TaskID:           taskID,
		UserID:           strings.TrimSpace(principal.UserID),
		WorkspaceID:      strings.TrimSpace(principal.WorkspaceID),
		StorageSettingID: strings.TrimSpace(settingID),
		ObjectKey:        objectKey,
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

	// 先把任务放到内存索引里，确保同一请求链后续上传分片时马上能拿到 task。
	svc.transferMu.Lock()
	svc.transferTasks[taskID] = task
	svc.transferMu.Unlock()
	// 任务刚创建时就先写主表，后续分片和状态迁移只更新这一条主记录。
	_ = svc.persistTransferTaskDB(ctx, task)
	// 再写 Redis 热快照，方便前端建完任务马上轮询或订阅到初始状态。
	svc.persistTransferSnapshot(task)
	return taskID
}

// UploadChunk 上传单个分片。
func (svc *FileService) UploadChunk(ctx context.Context, taskID string, partIndex int, chunk []byte, chunkHash string) (map[string]any, error) {
	// 先做最基本的入参校验，避免无意义的 DB/Redis 读写。
	if partIndex <= 0 {
		return nil, errors.New("chunkIndex must be positive")
	}
	if len(chunk) == 0 {
		return nil, errors.New("chunk is empty")
	}
	task := svc.ensureTransferTask(ctx, taskID)
	if task == nil {
		return nil, errors.New("transfer task not found")
	}
	// 状态先判一遍，避免暂停/取消后的新分片继续推进任务。
	if task.Status == filemodel.TransferTaskPaused {
		return nil, errors.New("transfer task is paused")
	}
	if task.Status == filemodel.TransferTaskCanceled {
		return nil, errors.New("transfer task is canceled")
	}
	if task.Status == filemodel.TransferTaskCompleted {
		// 已完成任务直接返回当前进度快照，不再允许重复推进分片。
		return transferStatusProgress(task), nil
	}
	if task.Status == filemodel.TransferTaskMerging {
		return nil, errors.New("transfer task is merging")
	}
	if partIndex > task.TotalParts {
		return nil, errors.New("chunkIndex exceeds total parts")
	}
	// 前端传过来的 chunkHash 只要提供了，就先做分片级校验。
	if strings.TrimSpace(chunkHash) != "" {
		// 分片 hash 对不上，直接拒绝，避免脏分片进入存储和任务表。
		if !strings.EqualFold(chunkHash, sha256Hex(chunk)) {
			return nil, errors.New("chunk hash mismatch")
		}
	}
	// 真正落分片前先给同一 task 同一 part 加短时锁，避免并发重复请求同时写底层存储和分片表。
	releaseLock, err := svc.acquireTransferPartLock(ctx, taskID, partIndex, 30*time.Second)
	if err != nil {
		return nil, err
	}
	// 只要走进关键区，无论成功还是失败都要释放分片锁，避免后续同片请求被错误卡死。
	defer releaseLock()

	partKey := transferPartRedisPrefix + taskID + ":" + fmt.Sprintf("%d", partIndex)
	if svc.rdb != nil {
		// 先看 Redis 去重 key，命中说明这片已经成功处理过，直接返回进度即可。
		if cached, err := svc.rdb.Get(ctx, partKey).Result(); err == nil && strings.TrimSpace(cached) != "" {
			var state map[string]any
			if jsonErr := json.Unmarshal([]byte(cached), &state); jsonErr == nil {
				return transferStatusProgress(task), nil
			}
		}
	}
	if svc.db != nil {
		var exists int64
		// Redis 没命中时再看分片表，处理 Redis 丢失或跨实例重复请求的情况。
		if err := svc.db.WithContext(ctx).Table("file_transfer_part").
			Where("task_id = ? AND part_index = ?", taskID, partIndex).
			Count(&exists).Error; err == nil && exists > 0 {
			return transferStatusProgress(task), nil
		}
	}

	objectKey := fmt.Sprintf("uploads/%s/%s/part-%d", time.Now().Format("20060102"), taskID, partIndex)
	if svc.storage != nil {
		// 分片先落底层存储，成功后再写任务主状态和分片表。
		if _, err := svc.storage.Put(ctx, storagemodel.ObjectPutInput{
			Key:           objectKey,
			Reader:        bytes.NewReader(chunk),
			ContentType:   "application/octet-stream",
			ContentLength: func() *int64 { v := int64(len(chunk)); return &v }(),
			Metadata: map[string]string{
				"task_id":    taskID,
				"part_index": fmt.Sprintf("%d", partIndex),
				"part_hash":  sha256Hex(chunk),
			},
		}); err != nil {
			return nil, err
		}
	}

	// 这里在真正写分片记录前再看一遍状态，处理“上传过程中用户刚好点了暂停/取消”的并发场景。
	svc.transferMu.Lock()
	if task.Status == filemodel.TransferTaskPaused {
		svc.transferMu.Unlock()
		return nil, errors.New("transfer task is paused")
	}
	if task.Status == filemodel.TransferTaskCanceled {
		svc.transferMu.Unlock()
		return nil, errors.New("transfer task is canceled")
	}
	if task.Status == filemodel.TransferTaskCompleted {
		svc.transferMu.Unlock()
		return transferStatusProgress(task), nil
	}
	if task.Chunks == nil {
		// 这里兜底初始化内存分片表，兼容从 DB 建任务但还没真正传过分片的场景。
		task.Chunks = make(map[int][]byte)
	}
	// 如果这片已经在内存里了，说明重复请求已经被之前的并发路径消化掉了。
	if _, ok := task.Chunks[partIndex]; ok {
		svc.transferMu.Unlock()
		return transferStatusProgress(task), nil
	}
	task.Chunks[partIndex] = append([]byte(nil), chunk...)
	svc.transferMu.Unlock()
	// 这里先写 DB 分片记录，成功后再刷新 Redis 热快照，DB 是最终依据。

	if svc.db != nil {
		if err := svc.persistTransferPartDB(ctx, task, partIndex, chunk, objectKey); err != nil {
			if isDuplicateTransferPartError(err) {
				// 唯一键冲突说明别的并发请求已经先成功写入这一片，这里把重复请求收敛成幂等成功。
				svc.transferMu.Lock()
				delete(task.Chunks, partIndex)
				task.UpdatedAt = time.Now()
				svc.transferMu.Unlock()
				if svc.storage != nil {
					// 本次重复请求若已经把对象写到底层，这里立即删除，避免留下孤儿重复分片。
					_ = svc.storage.Delete(ctx, objectKey)
				}
				if progressErr := svc.persistTransferProgressFromPartsWithVersion(ctx, task); progressErr != nil {
					return nil, progressErr
				}
				if svc.rdb != nil {
					// 既然分片事实已经存在，就同步补上 Redis 去重 key，让后续重复请求更快命中。
					snap, _ := json.Marshal(map[string]any{
						"partIndex": partIndex,
						"objectKey": objectKey,
						"status":    "uploaded",
					})
					_ = svc.rdb.Set(ctx, partKey, snap, 24*time.Hour).Err()
				}
				return transferStatusProgress(task), nil
			}
			// 分片表写失败要把内存暂存回滚，否则 merge 会看到一片“只有内存有、DB 没有”的脏状态。
			svc.transferMu.Lock()
			delete(task.Chunks, partIndex)
			task.UpdatedAt = time.Now()
			svc.transferMu.Unlock()
			if svc.storage != nil {
				// DB 没写成功时同步清掉刚才写入的对象，避免主事实失败却留下底层残片。
				_ = svc.storage.Delete(ctx, objectKey)
			}
			return nil, err
		}
	}
	// 分片成功后按分片表聚合值推进主任务进度，避免多个实例同时写分片时把计数互相覆盖。
	if err := svc.persistTransferProgressFromPartsWithVersion(ctx, task); err != nil {
		// 进度推进失败时把这次分片从内存移除，避免本地任务状态继续向前漂移。
		svc.transferMu.Lock()
		delete(task.Chunks, partIndex)
		task.UpdatedAt = time.Now()
		svc.transferMu.Unlock()
		if svc.storage != nil {
			// 主任务进度推进失败时也要回滚对象写入，避免对象事实和任务事实长期分叉。
			_ = svc.storage.Delete(ctx, objectKey)
		}
		return nil, err
	}
	if svc.rdb != nil {
		// Redis 只保存热点进度和分片去重 key，丢了可由 DB 重建。
		snap, _ := json.Marshal(map[string]any{
			"partIndex": partIndex,
			"objectKey": objectKey,
			"status":    "uploaded",
		})
		_ = svc.rdb.Set(ctx, partKey, snap, 24*time.Hour).Err()
	}
	// 最后返回的是更新后的任务快照，前端可以直接用它做低频轮询前的即时进度展示。
	return transferStatusProgress(task), nil
}

// MergeUpload 合并分片并落到存储层。
func (svc *FileService) MergeUpload(ctx context.Context, taskID string) (*filemodel.FileItem, error) {
	// merge 之前先看任务状态，暂停/取消/未完成都不允许进入合并。
	task := svc.ensureTransferTask(ctx, taskID)
	if task == nil {
		return nil, errors.New("transfer task not found")
	}
	if task.Status == filemodel.TransferTaskCanceled {
		return nil, errors.New("transfer task is canceled")
	}
	if task.Status == filemodel.TransferTaskPaused {
		return nil, errors.New("transfer task is paused")
	}
	if task.Status == filemodel.TransferTaskMerging {
		return nil, errors.New("transfer task is merging")
	}
	if len(task.Chunks) != task.TotalParts {
		// 进程重启或 Redis 丢失后，merge 前允许按 DB 分片记录回源底层存储补齐缺失 chunk。
		if err := svc.hydrateTransferTaskChunksFromStorage(ctx, task); err != nil {
			return nil, err
		}
	}
	if len(task.Chunks) != task.TotalParts {
		// 分片数不够时直接拒绝 merge，避免提前组装出脏对象。
		return nil, fmt.Errorf("chunks incomplete: %d/%d", len(task.Chunks), task.TotalParts)
	}

	// 分片按顺序拼起来。
	// 当前实现为了小改动继续沿用内存拼装；更优实践是本地存储走流式 merge，S3 走 multipart complete。
	ordered := make([][]byte, 0, task.TotalParts)
	for i := 1; i <= task.TotalParts; i++ {
		chunk, ok := task.Chunks[i]
		if !ok {
			// 分片索引有缺口时直接失败，避免出现“数量够但顺序缺片”的假完成。
			return nil, fmt.Errorf("missing chunk %d", i)
		}
		ordered = append(ordered, chunk)
	}
	// 这里把所有分片拼成一个连续 reader，方便统一计算整文件哈希并写入底层存储。
	merged := bytes.Join(ordered, nil)
	mergedHash := sha256Hex(merged)
	// merge 后再次校验整文件 hash，防止前端分片乱序、内容损坏或错误复用 task。
	if strings.TrimSpace(task.FileHash) != "" && !strings.EqualFold(strings.TrimSpace(task.FileHash), mergedHash) {
		return nil, errors.New("file hash mismatch after merge")
	}
	if strings.TrimSpace(task.FileHash) == "" {
		// 如果前端没给整文件 hash，就以 merge 后实际结果作为最终文件 hash。
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
	// 进入真正合并前，先把状态切成 MERGING，后续暂停/取消会被拒绝。
	if err := svc.StartMergeTransfer(ctx, task); err != nil {
		return nil, err
	}
	if svc.storage != nil {
		// S3 这类存储可以直接一次性写入合并后的对象，本地存储也走同一条接口。
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
	// 元数据入库前先在内存中准备最终文件对象。
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

	// 这里先准备副本给后续落库和返回，避免把共享内存对象直接交给调用方。
	cp := *item
	if err := svc.persistFileInfo(ctx, cp); err != nil {
		// 这里故意不把任务立刻改成 COMPLETED。
		// 原因是对象虽然已经写成功，但元数据还没成功落库，此时任务还不能算真正完成。
		// 这类失败交给补偿任务异步重试，后续重试成功后再把任务推进到终态更稳。
		// 这里登记 merge_retry 任务，后面 worker 可以按 task_id 继续补元数据。
		svc.registerMergeRetryJob(task.TaskID, err)
		return nil, fmt.Errorf("persist merge result failed: %w", err)
	}
	// 只有对象和元数据都成功后，任务才进入 COMPLETED，并登记临时分片清理任务。
	if err := svc.FinishMergeTransfer(ctx, task); err != nil {
		return nil, err
	}
	if svc.rdb != nil {
		// 终态任务不再需要 Redis 热快照，删掉让后续轮询回落到 DB/内存态。
		_ = svc.rdb.Del(ctx, transferTaskKey(taskID)).Err()
	}
	// 返回合并后的文件元数据，表示这次上传链路已经完成到“对象 + 元数据”这一层。
	return &cp, nil
}

// findWeakCandidates 在内存索引里按大小、后缀、存储配置筛秒传弱候选。
func (svc *FileService) findWeakCandidates(fileSize int64, suffix, settingID string) []*filemodel.FileItem {
	// 弱预检只依赖内存索引，所以这里用读锁保护遍历。
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	out := make([]*filemodel.FileItem, 0)
	for _, item := range svc.items {
		if item.Deleted || item.IsDir {
			// 弱预检候选只看有效文件，不把目录和已删除项算进去。
			continue
		}
		if strings.TrimSpace(settingID) != "" && item.StorageSettingID != strings.TrimSpace(settingID) {
			// 指定存储配置时只在该配置下筛候选，避免跨存储误命中。
			continue
		}
		if item.Size != fileSize {
			// 弱预检第一层直接按文件大小筛掉大部分无关项。
			continue
		}
		if fileSuffix(item.Name) != suffix {
			// 后缀也要一致，减少后续强预检哈希比对的候选数。
			continue
		}
		out = append(out, item)
	}
	return out
}

// cloneInstantFile 在内存模式下为秒传命中的对象复制一条新文件记录。
func (svc *FileService) cloneInstantFile(src *filemodel.FileItem, parentID, settingID string) *filemodel.FileItem {
	// 秒传命中时只复制元数据，所以这里修改的是文件索引而不是底层对象内容。
	svc.mu.Lock()
	defer svc.mu.Unlock()

	now := time.Now()
	id := svc.nextIDLocked()
	// 即使复用同一对象内容，也要在目标目录里生成一个不冲突的新名字。
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
	// 把新文件挂进共享索引，后续列表和详情查询都能看到这条秒传生成的记录。
	svc.items[id] = item
	cp := *item
	return &cp
}

// precheckUploadDB 在 DB 模式下执行弱预检、强预检和任务创建。
func (svc *FileService) precheckUploadDB(ctx context.Context, in filemodel.UploadInitInput, settingID string) (filemodel.UploadPrecheckResult, error) {
	// DB 预检同样先解析主体，保证候选只在当前用户当前空间里找。
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
		// DB 弱预检也支持按存储配置进一步收窄候选范围。
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
			// 强预检命中后只复制元数据，不重复上传对象内容。
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

	// 走到这里说明没有命中秒传，预检阶段改为创建上传任务。
	taskID := svc.initTransferTask(ctx, in, settingID)
	result.TaskID = taskID
	result.UploadID = taskID
	// DB 模式同样返回任务初始快照，保证 precheck-progress 不再依赖前端本地兜底。
	result.Progress = svc.GetTransferTaskSnapshot(taskID)
	return result, nil
}

// cloneInstantFileDB 在 DB 模式下为秒传命中的对象复制一条新元数据记录。
func (svc *FileService) cloneInstantFileDB(ctx context.Context, userID, workspaceID string, src *filedb.FileInfo, parentID, settingID string) (*filemodel.FileItem, error) {
	// DB 秒传复制前先在目标目录里生成可用名称，避免展示层直接出现同名冲突。
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
	// 这里直接插入一条新文件元数据，object_key 复用历史对象，不触发新的存储写入。
	if err = svc.db.WithContext(ctx).Table("file_info").Create(insert).Error; err != nil {
		return nil, err
	}
	// 返回新文件 DTO，表示这次秒传已经在目录树上创建了一个新的文件入口。
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

// uniqueNameDB 在 DB 模式下为目标目录生成一个不冲突的显示名称。
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
		// 这里循环查同目录同名数，直到找到一个没被占用的 display_name。
		if err := svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
			Where("user_id = ? AND workspace_id = ? AND is_deleted = 0 AND display_name = ?", userID, workspaceID, current).
			Where("(parent_id = ? OR (? = '' AND (parent_id = '' OR parent_id IS NULL OR parent_id = 'root' OR parent_id = 'ROOT')))", parent, parent).
			Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return current, nil
		}
		// 一旦重名，就按常见 "(n)" 规则继续尝试下一个名称。
		current = fmt.Sprintf("%s(%d)", base, idx)
		idx++
	}
}

// sha256Hex 统一计算字节内容的 SHA-256 十六进制摘要。
func sha256Hex(data []byte) string {
	// 上传链路统一使用 SHA-256，避免不同阶段用不同哈希算法带来判重歧义。
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// isDuplicateTransferPartError 判断分片唯一键冲突，避免把重复分片请求错误地当成上传失败。
func isDuplicateTransferPartError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	// 当前项目没有统一打开驱动级错误翻译，因此这里兼容 MySQL 常见重复键报文。
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate entry") || strings.Contains(msg, "1062") || strings.Contains(msg, "duplicated key")
}
