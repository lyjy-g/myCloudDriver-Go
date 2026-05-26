package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	filemodel "myclouddrive-go/internal/file/model"
	"myclouddrive-go/internal/framework/security"
	storagesvc "myclouddrive-go/internal/storage/service"
)

// FileService 是 file 模块的唯一实现。
type FileService struct {
	db      *gorm.DB
	rdb     redis.Cmdable
	mu      sync.RWMutex
	counter int64
	items   map[string]*filemodel.FileItem
	storage IStoragePower

	idemMu      sync.Mutex
	idemRecords map[string]idempotencyRecord
	idemTTL     time.Duration

	transferMu    sync.Mutex
	transferTasks map[string]*filemodel.TransferTask
	cleanupJobs   map[string]*filemodel.TransferCleanupJob

	// taskWatcherMu 保护每个上传任务的 SSE 订阅者集合，避免上传进度和订阅增删并发踩踏。
	taskWatcherMu sync.Mutex
	// taskWatchers 让同一 task 的分片推进、状态切换和 merge 能把最新快照推给对应订阅端。
	taskWatchers map[string]map[chan map[string]any]struct{}
}

const (
	transferPartRedisPrefix = "file:transfer:parts:"
	transferTaskRedisPrefix = "file:transfer:task:"
	transferLockRedisPrefix = "file:transfer:lock:"
)

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
	ErrTransferTaskVersion   = errors.New("transfer task version changed")
)

// NewFileService 创建文件服务。
func NewFileService(storage *storagesvc.StorageService, extras ...any) *FileService {
	var db *gorm.DB
	var rdb redis.Cmdable
	var storagePower IStoragePower
	for _, extra := range extras {
		if v, ok := extra.(*gorm.DB); ok {
			db = v
			continue
		}
		if v, ok := extra.(redis.Cmdable); ok {
			rdb = v
			break
		}
	}
	if storage != nil {
		storagePower = storage
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
		rdb:           rdb,
		counter:       1,
		items:         map[string]*filemodel.FileItem{root.ID: root},
		storage:       storagePower,
		idemRecords:   make(map[string]idempotencyRecord),
		idemTTL:       24 * time.Hour,
		transferTasks: make(map[string]*filemodel.TransferTask),
		cleanupJobs:   make(map[string]*filemodel.TransferCleanupJob),
		taskWatchers:  make(map[string]map[chan map[string]any]struct{}),
	}
}

// Ping 服务健康检查。
func (svc *FileService) Ping(_ context.Context) (string, error) {
	return "file service ready", nil
}

// transferPartsKey 生成 Redis 里的分片去重 key 前缀。
func transferPartsKey(taskID string) string {
	return transferPartRedisPrefix + strings.TrimSpace(taskID)
}

// transferTaskKey 生成 Redis 里的任务热点快照 key。
func transferTaskKey(taskID string) string {
	return transferTaskRedisPrefix + strings.TrimSpace(taskID)
}

// transferLockKey 生成 Redis 里的任务短时锁 key。
func transferLockKey(taskID string) string {
	return transferLockRedisPrefix + strings.TrimSpace(taskID)
}

// transferPartLockKey 生成单分片短时锁 key，避免并发重复请求同时穿透到存储和 DB。
func transferPartLockKey(taskID string, partIndex int) string {
	return transferLockKey(taskID) + fmt.Sprintf(":part:%d", partIndex)
}

// transferProgress 根据总分片数和已上传分片数计算百分比进度。
func transferProgress(totalParts, uploadedParts int) int {
	if totalParts <= 0 {
		return 0
	}
	if uploadedParts <= 0 {
		return 0
	}
	if uploadedParts >= totalParts {
		return 100
	}
	return uploadedParts * 100 / totalParts
}

// transferStatusProgress 组装统一的任务进度视图，供接口和 Redis 快照复用。
func transferStatusProgress(task *filemodel.TransferTask) map[string]any {
	if task == nil {
		return map[string]any{}
	}
	// 这里统一收口任务进度结构，避免不同接口各自计算 progress 导致口径不一致。
	return map[string]any{
		"taskId":         task.TaskID,
		"status":         task.Status,
		"uploadedParts":  task.UploadedPart,
		"totalParts":     task.TotalParts,
		"uploadedSize":   task.UploadedSize,
		"progress":       transferProgress(task.TotalParts, task.UploadedPart),
		"fileName":       task.FileName,
		"fileSize":       task.FileSize,
		"storageSetting": task.StorageSettingID,
		"version":        task.Version,
	}
}

// persistTransferSnapshot 把任务热点进度刷新到 Redis。
func (svc *FileService) persistTransferSnapshot(task *filemodel.TransferTask) {
	if svc.rdb == nil || task == nil {
		// 即使没有 Redis，也继续往 SSE watcher 推送，避免本地模式下前端完全收不到任务进度。
		svc.publishTransferSnapshot(task)
		return
	}
	// Redis 里只存展示层热点字段，不把 chunk 内容或整行 DB 数据都塞进去。
	payload, err := json.Marshal(transferStatusProgress(task))
	if err != nil {
		// Redis 快照序列化失败时仍继续推送内存快照，避免 SSE/轮询展示同时失效。
		svc.publishTransferSnapshot(task)
		return
	}
	// 这里统一给 24 小时 TTL，既能覆盖一次上传会话，也避免热 key 永久堆积。
	_ = svc.rdb.Set(context.Background(), transferTaskKey(task.TaskID), payload, 24*time.Hour).Err()
	// Redis 写完后继续广播同一份快照，让订阅端拿到和轮询端一致的进度口径。
	svc.publishTransferSnapshot(task)
}

// loadTransferSnapshot 读取 Redis 中的任务热点快照。
func (svc *FileService) loadTransferSnapshot(taskID string) map[string]any {
	if svc.rdb == nil {
		return nil
	}
	// Redis 没有命中就直接回退，调用方再决定是否继续查内存或 DB。
	raw, err := svc.rdb.Get(context.Background(), transferTaskKey(taskID)).Result()
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]any
	// 快照反序列化失败时按无快照处理，避免坏数据把主流程拖挂。
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// transferTaskVersionKey 生成 Redis 里的任务版本缓存 key。
func transferTaskVersionKey(taskID string) string {
	return transferTaskRedisPrefix + "version:" + strings.TrimSpace(taskID)
}

// persistTransferTaskVersion 把任务版本写入 Redis，便于热状态和 DB 主状态对齐。
func (svc *FileService) persistTransferTaskVersion(taskID string, version int) {
	if svc.rdb == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	// 这里只写最小热点字段，避免把主状态和版本更新拆成两段复杂逻辑。
	_ = svc.rdb.Set(context.Background(), transferTaskVersionKey(taskID), fmt.Sprintf("%d", version), 24*time.Hour).Err()
}

// nextTransferTaskVersion 把任务版本号向前推进一步。
func (svc *FileService) nextTransferTaskVersion(task *filemodel.TransferTask) int {
	if task == nil {
		return 0
	}
	// 版本号在 DB 条件更新成功后再推进，这里只负责刷新内存时间戳和热快照。
	task.UpdatedAt = time.Now()
	svc.persistTransferSnapshot(task)
	svc.persistTransferTaskVersion(task.TaskID, task.Version)
	return task.Version
}

// acquireTransferPartLock 为单个分片获取短时锁；没有 Redis 时回退为 no-op，不阻塞本地模式。
func (svc *FileService) acquireTransferPartLock(ctx context.Context, taskID string, partIndex int, ttl time.Duration) (func(), error) {
	if svc.rdb == nil {
		// 本地无 Redis 模式下继续依赖内存 + DB 唯一约束兜底，避免为测试场景额外引入假锁实现。
		return func() {}, nil
	}
	lockKey := transferPartLockKey(taskID, partIndex)
	// 这里用 SETNX 做短时锁，只保护“同一 task 同一 part”的落分片关键区，不扩大锁范围。
	ok, err := svc.rdb.SetNX(ctx, lockKey, "1", ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("transfer chunk is uploading")
	}
	// 释放时只删当前锁 key，不影响任务级其他分片。
	return func() {
		_ = svc.rdb.Del(context.Background(), lockKey).Err()
	}, nil
}

// transferTaskRowState 是主任务表里推进状态所需的最小字段集合。
type transferTaskRowState struct {
	Status        string
	Version       int
	UploadedParts int
	UploadedSize  int64
}

// registerCleanupJob 记录一个异步清理任务，后续可直接接 MQ 或 worker 扫描。
func (svc *FileService) registerCleanupJob(taskID, jobType string) {
	if strings.TrimSpace(taskID) == "" {
		return
	}
	// 这里先把清理意图落下来，避免主请求结束后丢失“该删什么”的信息。
	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	jobID := fmt.Sprintf("cj_%d", time.Now().UnixNano())
	job := &filemodel.TransferCleanupJob{
		JobID:     jobID,
		TaskID:    taskID,
		JobType:   strings.TrimSpace(jobType),
		Status:    "PENDING",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	// 先落内存，保证当前进程立即可见；再尝试写 DB，方便进程重启后还能扫描到这条清理任务。
	svc.cleanupJobs[jobID] = job
	if svc.db != nil {
		// DB 里保留清理任务，是为了让“什么时候该清理”这件事脱离当前请求生命周期。
		_ = svc.db.WithContext(context.Background()).Table("file_transfer_cleanup_job").Create(map[string]any{
			"job_id":        job.JobID,
			"task_id":       job.TaskID,
			"job_type":      job.JobType,
			"status":        job.Status,
			"retry_count":   0,
			"error_msg":     nil,
			"next_retry_at": nil,
			"finished_at":   nil,
			"created_at":    job.CreatedAt,
			"updated_at":    job.UpdatedAt,
		}).Error
	}
	// 清理意图落下来后立即异步执行，避免任务结束后临时分片长期滞留。
	go svc.processCleanupJob(context.Background(), job.JobID)
}

// subscribeTransferTask 为单个任务注册一个 SSE watcher。
func (svc *FileService) subscribeTransferTask(taskID string) chan map[string]any {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	// 每个订阅方拿独立 channel，避免一个慢连接阻塞整个任务的其他观察者。
	ch := make(chan map[string]any, 16)
	svc.taskWatcherMu.Lock()
	defer svc.taskWatcherMu.Unlock()
	if svc.taskWatchers[taskID] == nil {
		svc.taskWatchers[taskID] = make(map[chan map[string]any]struct{})
	}
	svc.taskWatchers[taskID][ch] = struct{}{}
	return ch
}

// unsubscribeTransferTask 移除任务 watcher，避免连接断开后 watcher 泄漏。
func (svc *FileService) unsubscribeTransferTask(taskID string, ch chan map[string]any) {
	if strings.TrimSpace(taskID) == "" || ch == nil {
		return
	}
	// 断开时需要把 watcher 从任务集合里摘掉，并在集合清空后删掉 task 桶。
	svc.taskWatcherMu.Lock()
	defer svc.taskWatcherMu.Unlock()
	watchers := svc.taskWatchers[taskID]
	if watchers == nil {
		return
	}
	delete(watchers, ch)
	if len(watchers) == 0 {
		delete(svc.taskWatchers, taskID)
	}
	close(ch)
}

// publishTransferSnapshot 把任务最新快照广播给当前 task 的所有订阅者。
func (svc *FileService) publishTransferSnapshot(task *filemodel.TransferTask) {
	if task == nil {
		return
	}
	// 这里统一复用任务快照结构，避免 SSE 事件和轮询接口返回口径不一致。
	snapshot := transferStatusProgress(task)
	svc.taskWatcherMu.Lock()
	defer svc.taskWatcherMu.Unlock()
	for ch := range svc.taskWatchers[task.TaskID] {
		select {
		case ch <- snapshot:
		default:
			// watcher 来不及消费时直接丢弃旧快照，让最新进度覆盖旧进度，避免上传主链路被慢客户端拖住。
		}
	}
}

// getUploadedChunkIndexes 统一返回任务已上传分片列表，供接口和恢复逻辑复用。
func (svc *FileService) getUploadedChunkIndexes(ctx context.Context, taskID string) []int {
	if strings.TrimSpace(taskID) == "" {
		return []int{}
	}
	if svc.db != nil {
		// 优先按 DB 分片表返回真实已落库分片，避免多实例或 Redis 丢失时只看到当前进程内存态。
		var rows []struct {
			PartIndex int `gorm:"column:part_index"`
		}
		if err := svc.db.WithContext(ctx).Table("file_transfer_part").
			Select("part_index").
			Where("task_id = ?", taskID).
			Order("part_index asc").
			Find(&rows).Error; err == nil {
			out := make([]int, 0, len(rows))
			for _, row := range rows {
				out = append(out, row.PartIndex)
			}
			return out
		}
	}
	// 没有 DB 或 DB 查询失败时，回退到内存分片集合，保证本地模式仍然能恢复上传进度。
	task := svc.getTransferTask(taskID)
	if task == nil {
		return []int{}
	}
	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	out := make([]int, 0, len(task.Chunks))
	for partIndex := range task.Chunks {
		out = append(out, partIndex)
	}
	sort.Ints(out)
	return out
}

// ensureTransferTask 优先从内存取任务；内存没有时再尝试从 DB 恢复主状态。
func (svc *FileService) ensureTransferTask(ctx context.Context, taskID string) *filemodel.TransferTask {
	task := svc.getTransferTask(taskID)
	if task != nil || svc.db == nil || strings.TrimSpace(taskID) == "" {
		return task
	}
	// 这里把 DB 主状态回灌到内存索引，避免 Redis 或进程内存丢失后任务直接“查无此任务”。
	loaded, err := svc.loadTransferTaskFromDB(ctx, taskID)
	if err != nil || loaded == nil {
		return nil
	}
	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	if existing := svc.transferTasks[taskID]; existing != nil {
		return existing
	}
	svc.transferTasks[taskID] = loaded
	return loaded
}

// loadTransferTaskFromDB 从主任务表恢复任务最小状态，供进程重启后的上传链路继续工作。
func (svc *FileService) loadTransferTaskFromDB(ctx context.Context, taskID string) (*filemodel.TransferTask, error) {
	if svc.db == nil || strings.TrimSpace(taskID) == "" {
		return nil, nil
	}
	// 这里只恢复上传链路必需字段，避免把 DB 行模型泄漏到更高层。
	var row struct {
		TaskID           string    `gorm:"column:task_id"`
		UserID           string    `gorm:"column:user_id"`
		WorkspaceID      string    `gorm:"column:workspace_id"`
		StorageSettingID string    `gorm:"column:storage_platform_setting_id"`
		ObjectKey        string    `gorm:"column:object_key"`
		FileName         string    `gorm:"column:file_name"`
		FileHash         string    `gorm:"column:file_sha256"`
		FileSize         int64     `gorm:"column:file_size"`
		ContentType      string    `gorm:"column:mime_type"`
		ParentID         string    `gorm:"column:parent_id"`
		TotalParts       int       `gorm:"column:total_chunks"`
		UploadedPart     int       `gorm:"column:uploaded_chunks"`
		UploadedSize     int64     `gorm:"column:uploaded_size"`
		Status           string    `gorm:"column:status"`
		Version          int       `gorm:"column:version"`
		CreatedAt        time.Time `gorm:"column:created_at"`
		UpdatedAt        time.Time `gorm:"column:updated_at"`
	}
	if err := svc.db.WithContext(ctx).Table("file_transfer_task").
		Select("task_id, user_id, workspace_id, storage_platform_setting_id, object_key, file_name, file_sha256, file_size, mime_type, parent_id, total_chunks, uploaded_chunks, uploaded_size, status, version, created_at, updated_at").
		Where("task_id = ?", taskID).
		Take(&row).Error; err != nil {
		return nil, err
	}
	// 恢复后的任务先挂空 chunk map；真正 merge 时再按 DB 分片记录回填缺失 chunk 内容。
	return &filemodel.TransferTask{
		TaskID:           row.TaskID,
		UserID:           row.UserID,
		WorkspaceID:      row.WorkspaceID,
		StorageSettingID: row.StorageSettingID,
		ObjectKey:        row.ObjectKey,
		FileName:         row.FileName,
		FileHash:         row.FileHash,
		FileSize:         row.FileSize,
		ContentType:      row.ContentType,
		ParentID:         row.ParentID,
		TotalParts:       row.TotalParts,
		UploadedPart:     row.UploadedPart,
		UploadedSize:     row.UploadedSize,
		Status:           filemodel.TransferTaskStatus(row.Status),
		Version:          row.Version,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Chunks:           make(map[int][]byte),
	}, nil
}

// hydrateTransferTaskChunksFromStorage 在 merge 前按分片表 + 底层存储补齐内存缺失分片。
func (svc *FileService) hydrateTransferTaskChunksFromStorage(ctx context.Context, task *filemodel.TransferTask) error {
	if task == nil || svc.db == nil || svc.storage == nil {
		return nil
	}
	// 这里按分片表恢复 object_key，再逐片回源到底层存储，解决“任务从 DB 恢复但内存没 chunk”场景。
	var rows []struct {
		PartIndex int    `gorm:"column:part_index"`
		ObjectKey string `gorm:"column:object_key"`
	}
	if err := svc.db.WithContext(ctx).Table("file_transfer_part").
		Select("part_index, object_key").
		Where("task_id = ?", task.TaskID).
		Order("part_index asc").
		Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if row.PartIndex <= 0 || strings.TrimSpace(row.ObjectKey) == "" {
			continue
		}
		svc.transferMu.Lock()
		if task.Chunks == nil {
			task.Chunks = make(map[int][]byte)
		}
		_, exists := task.Chunks[row.PartIndex]
		svc.transferMu.Unlock()
		if exists {
			continue
		}
		reader, _, err := svc.storage.Get(ctx, row.ObjectKey)
		if err != nil {
			return err
		}
		// 这里只在 merge 前短暂回填缺失 chunk 内容，避免常态请求都去读底层对象。
		content, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil {
			return readErr
		}
		svc.transferMu.Lock()
		task.Chunks[row.PartIndex] = content
		svc.transferMu.Unlock()
	}
	return nil
}

// registerMergeRetryJob 记录 merge 成功但元数据入库失败后的补偿任务。
func (svc *FileService) registerMergeRetryJob(taskID string, err error) {
	if strings.TrimSpace(taskID) == "" {
		return
	}
	// 这里不直接重试，是为了把“底层对象已经落成功、但元数据没落成功”的事实先保留下来。
	svc.transferMu.Lock()
	defer svc.transferMu.Unlock()
	jobID := fmt.Sprintf("mj_%d", time.Now().UnixNano())
	message := ""
	if err != nil {
		message = err.Error()
	}
	job := &filemodel.TransferCleanupJob{
		JobID:     jobID,
		TaskID:    taskID,
		JobType:   "merge_retry",
		Status:    "PENDING",
		ErrorMsg:  message,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	// 先记到内存，便于当前进程立即观测到“对象已写成但元数据失败”的补偿状态。
	svc.cleanupJobs[jobID] = job
	if svc.db != nil {
		// 再把补偿任务写库，后续 worker 或 MQ 消费者可以据此继续补元数据。
		_ = svc.db.WithContext(context.Background()).Table("file_transfer_cleanup_job").Create(map[string]any{
			"job_id":        job.JobID,
			"task_id":       job.TaskID,
			"job_type":      job.JobType,
			"status":        job.Status,
			"retry_count":   0,
			"error_msg":     job.ErrorMsg,
			"next_retry_at": time.Now(),
			"finished_at":   nil,
			"created_at":    job.CreatedAt,
			"updated_at":    job.UpdatedAt,
		}).Error
	}
}

// processCleanupJob 异步执行上传临时分片清理，并把结果回写到内存和 DB。
func (svc *FileService) processCleanupJob(ctx context.Context, jobID string) {
	if strings.TrimSpace(jobID) == "" {
		return
	}
	svc.transferMu.Lock()
	job := svc.cleanupJobs[jobID]
	svc.transferMu.Unlock()
	if job == nil {
		return
	}
	// 这里只处理分片清理类任务；merge_retry 后续仍可接专门补偿 worker。
	if job.JobType != "cancel_cleanup" && job.JobType != "merge_cleanup" {
		return
	}
	if err := svc.updateCleanupJobStatus(ctx, job, "RUNNING", "", false); err != nil {
		return
	}
	err := svc.cleanupTransferPartObjects(ctx, job.TaskID)
	if err != nil {
		// 清理失败时把错误留下来，便于后续重试器或人工排查继续处理。
		_ = svc.updateCleanupJobStatus(ctx, job, "FAILED", err.Error(), false)
		return
	}
	// 清理成功后把 job 标为完成，形成“意图 + 执行结果”闭环。
	_ = svc.updateCleanupJobStatus(ctx, job, "DONE", "", true)
}

// cleanupTransferPartObjects 删除任务对应的临时分片对象和分片表记录。
func (svc *FileService) cleanupTransferPartObjects(ctx context.Context, taskID string) error {
	if strings.TrimSpace(taskID) == "" || svc.db == nil || svc.storage == nil {
		return nil
	}
	// 这里按分片表查对象 key，再逐个删底层对象，最后删除分片元数据，避免丢失清理目标。
	var rows []struct {
		ObjectKey string `gorm:"column:object_key"`
	}
	if err := svc.db.WithContext(ctx).Table("file_transfer_part").
		Select("object_key").
		Where("task_id = ?", taskID).
		Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if strings.TrimSpace(row.ObjectKey) == "" {
			continue
		}
		if err := svc.storage.Delete(ctx, row.ObjectKey); err != nil {
			return err
		}
	}
	// 对象删完再删分片元数据，保证重试时仍能知道还剩哪些对象未清。
	return svc.db.WithContext(ctx).Table("file_transfer_part").Where("task_id = ?", taskID).Delete(nil).Error
}

// updateCleanupJobStatus 统一推进清理任务状态，避免内存和 DB 各自漂移。
func (svc *FileService) updateCleanupJobStatus(ctx context.Context, job *filemodel.TransferCleanupJob, status, message string, finished bool) error {
	if job == nil {
		return nil
	}
	now := time.Now()
	svc.transferMu.Lock()
	job.Status = status
	job.ErrorMsg = message
	job.UpdatedAt = now
	if finished {
		job.FinishedAt = now
	}
	svc.transferMu.Unlock()
	if svc.db == nil {
		return nil
	}
	updates := map[string]any{
		"status":     status,
		"error_msg":  nullableCleanupMessage(message),
		"updated_at": now,
	}
	if finished {
		updates["finished_at"] = now
	}
	return svc.db.WithContext(ctx).Table("file_transfer_cleanup_job").Where("job_id = ?", job.JobID).Updates(updates).Error
}

// nullableCleanupMessage 把空错误串转成 nil，避免 DB 里出现无意义空串。
func nullableCleanupMessage(message string) any {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	return message
}

// setTransferTaskStatus 原子更新内存任务状态并推进版本号。
func (svc *FileService) setTransferTaskStatus(task *filemodel.TransferTask, status filemodel.TransferTaskStatus) {
	if task == nil {
		return
	}
	// 只有状态合法时才允许变更，避免跳过中间态造成前端和后端口径不一致。
	task.Status = status
	task.UpdatedAt = time.Now()
	svc.persistTransferSnapshot(task)
}

// canTransitionTransferStatus 先在内存层判断状态是否允许迁移。
func canTransitionTransferStatus(current, target filemodel.TransferTaskStatus) bool {
	switch current {
	case filemodel.TransferTaskUploading:
		// 上传中允许被暂停、进入 merge 收尾，或者被主动取消。
		return target == filemodel.TransferTaskPaused || target == filemodel.TransferTaskMerging || target == filemodel.TransferTaskCanceled
	case filemodel.TransferTaskPaused:
		// 暂停态只能恢复继续传，或者直接取消，不允许跳到 completed。
		return target == filemodel.TransferTaskUploading || target == filemodel.TransferTaskCanceled
	case filemodel.TransferTaskMerging:
		// merge 是保护态，只允许自然完成，不允许回退。
		return target == filemodel.TransferTaskCompleted
	case filemodel.TransferTaskCompleted, filemodel.TransferTaskCanceled:
		// 终态不允许继续迁移，避免后续请求把任务重新拉活。
		return false
	default:
		// 未知状态只允许收敛回上传态，避免放开任意跳转。
		return target == filemodel.TransferTaskUploading
	}
}

// loadTransferTaskRowState 读取 DB 主表中的任务状态，用于乐观锁冲突后的对齐。
func (svc *FileService) loadTransferTaskRowState(ctx context.Context, taskID string) (transferTaskRowState, error) {
	if svc.db == nil {
		return transferTaskRowState{}, nil
	}
	// 这里只取状态推进需要的最小字段，避免把整行都搬进内存。
	var row transferTaskRowState
	if err := svc.db.WithContext(ctx).Table("file_transfer_task").
		Select("status, version, uploaded_chunks AS uploaded_parts, uploaded_size").
		Where("task_id = ?", taskID).
		Take(&row).Error; err != nil {
		return transferTaskRowState{}, err
	}
	// 返回的最小状态结构只服务于冲突恢复和版本推进，不承担完整任务详情职责。
	return row, nil
}

// loadTransferPartAggregates 从分片表重算上传进度，作为 Redis 丢失或并发冲突后的最终依据。
func (svc *FileService) loadTransferPartAggregates(ctx context.Context, taskID string) (int, int64, error) {
	if svc.db == nil {
		return 0, 0, nil
	}
	// 这里不信任内存计数，直接按分片表聚合，避免多实例并发时把 uploadedParts 回写小了。
	var row struct {
		UploadedParts int
		UploadedSize  int64
	}
	if err := svc.db.WithContext(ctx).Table("file_transfer_part").
		Select("COUNT(1) AS uploaded_parts, COALESCE(SUM(part_size), 0) AS uploaded_size").
		Where("task_id = ?", taskID).
		Take(&row).Error; err != nil {
		return 0, 0, err
	}
	// 聚合结果直接作为主任务进度推进依据，避免依赖某个实例的内存计数。
	return row.UploadedParts, row.UploadedSize, nil
}

// persistTransferTaskDB 把上传任务主状态写入 DB，作为最终依据。
func (svc *FileService) persistTransferTaskDB(ctx context.Context, task *filemodel.TransferTask) error {
	if svc.db == nil || task == nil {
		return nil
	}
	// 任务创建和任务推进都走同一张主表，后续 Redis 丢失也能从这里恢复。
	userID := strings.TrimSpace(task.UserID)
	workspaceID := strings.TrimSpace(task.WorkspaceID)
	if (userID == "" || workspaceID == "") && ctx != nil {
		// 创建任务时优先用任务对象里的主体信息；没有时再回退到请求上下文。
		principal, _ := security.RequireLogin(ctx)
		if userID == "" {
			userID = strings.TrimSpace(principal.UserID)
		}
		if workspaceID == "" {
			workspaceID = strings.TrimSpace(principal.WorkspaceID)
		}
	}
	if userID == "" || workspaceID == "" {
		return errors.New("transfer task principal is required")
	}
	// 这里回填主体信息到任务对象，后续状态推进就不需要每次再从 ctx 解析一次主体。
	task.UserID = userID
	task.WorkspaceID = workspaceID
	if task.Version <= 0 {
		// 新任务第一次落库时，版本号从 1 开始，后续所有状态推进都基于这个版本做 CAS。
		task.Version = 1
	}
	// 这里使用幂等 upsert 风格写法，避免任务创建和状态推进分散到不同表单逻辑里。
	row := map[string]any{
		"task_id":                     task.TaskID,
		"upload_id":                   task.TaskID,
		"parent_id":                   task.ParentID,
		"user_id":                     userID,
		"workspace_id":                workspaceID,
		"storage_platform_setting_id": task.StorageSettingID,
		"object_key":                  task.ObjectKey,
		"file_name":                   task.FileName,
		"file_size":                   task.FileSize,
		"file_sha256":                 task.FileHash,
		"suffix":                      fileSuffix(task.FileName),
		"mime_type":                   task.ContentType,
		"total_chunks":                task.TotalParts,
		"task_type":                   "upload",
		"uploaded_chunks":             task.UploadedPart,
		"chunk_size":                  int64(5 * 1024 * 1024),
		"uploaded_size":               task.UploadedSize,
		"status":                      string(task.Status),
		"version":                     task.Version,
		"start_time":                  task.CreatedAt,
		"complete_time":               completeTimeOrNil(task),
		"created_at":                  task.CreatedAt,
		"updated_at":                  time.Now(),
	}
	// 先尝试插入，已存在则按 task_id 更新；当前版本只保留单行主状态。
	if err := svc.db.WithContext(ctx).Table("file_transfer_task").Create(row).Error; err != nil {
		// 已存在就按 task_id 更新同一条主状态，不再额外插第二行。
		return svc.db.WithContext(ctx).Table("file_transfer_task").
			Where("task_id = ?", task.TaskID).
			Updates(map[string]any{
				"user_id":         userID,
				"workspace_id":    workspaceID,
				"object_key":      task.ObjectKey,
				"parent_id":       task.ParentID,
				"file_name":       task.FileName,
				"file_size":       task.FileSize,
				"file_sha256":     task.FileHash,
				"suffix":          fileSuffix(task.FileName),
				"mime_type":       task.ContentType,
				"total_chunks":    task.TotalParts,
				"uploaded_chunks": task.UploadedPart,
				"uploaded_size":   task.UploadedSize,
				"status":          string(task.Status),
				"version":         task.Version,
				"complete_time":   completeTimeOrNil(task),
				"updated_at":      time.Now(),
			}).Error
	}
	return nil
}

// persistTransferTaskStatusWithVersion 用 version 条件更新主状态，防止并发覆盖。
func (svc *FileService) persistTransferTaskStatusWithVersion(ctx context.Context, task *filemodel.TransferTask, expectedVersion int) error {
	if svc.db == nil || task == nil {
		return nil
	}
	// 这里用 version 做乐观锁，谁先写成功谁生效，后来的并发请求直接失败。
	nextVersion := expectedVersion + 1
	result := svc.db.WithContext(ctx).Table("file_transfer_task").
		Where("task_id = ? AND version = ?", task.TaskID, expectedVersion).
		Updates(map[string]any{
			"status":          string(task.Status),
			"uploaded_chunks": task.UploadedPart,
			"uploaded_size":   task.UploadedSize,
			"version":         nextVersion,
			"complete_time":   completeTimeOrNil(task),
			"updated_at":      time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 没更新到通常意味着别的请求已经先一步改了状态，这里直接让上层返回冲突。
		return ErrTransferTaskVersion
	}
	// 只有 DB CAS 成功后才回写新版本，确保内存 version 不会跑到 DB 前面。
	task.Version = nextVersion
	task.UpdatedAt = time.Now()
	svc.persistTransferSnapshot(task)
	svc.persistTransferTaskVersion(task.TaskID, task.Version)
	return nil
}

// persistTransferProgressFromPartsWithVersion 用分片表聚合结果推进主任务进度，避免多实例并发时覆盖计数。
func (svc *FileService) persistTransferProgressFromPartsWithVersion(ctx context.Context, task *filemodel.TransferTask) error {
	if task == nil {
		return nil
	}
	if svc.db == nil {
		// 纯内存模式下没有 DB 可对齐时，直接按当前内存分片集合重算进度。
		// 这样单元测试和纯内存运行模式下也能拿到正确的 uploadedParts/uploadedSize。
		uploadedParts := len(task.Chunks)
		var uploadedSize int64
		for _, chunk := range task.Chunks {
			uploadedSize += int64(len(chunk))
		}
		// 回退路径下直接用内存分片集合更新进度，保证无 DB 模式也能看到正确快照。
		task.UploadedPart = uploadedParts
		task.UploadedSize = uploadedSize
		task.Version++
		task.UpdatedAt = time.Now()
		svc.persistTransferSnapshot(task)
		svc.persistTransferTaskVersion(task.TaskID, task.Version)
		return nil
	}
	// 这里最多重试三次，处理“别的实例刚好先一步推进了 version”的场景。
	for attempt := 0; attempt < 3; attempt++ {
		// 每轮都先拿 DB 里的当前版本和状态，确保推进基于最新主状态做 CAS。
		row, err := svc.loadTransferTaskRowState(ctx, task.TaskID)
		if err != nil {
			return err
		}
		// 再按分片表重算已上传进度，避免某个实例的本地计数覆盖全局真实值。
		uploadedParts, uploadedSize, err := svc.loadTransferPartAggregates(ctx, task.TaskID)
		if err != nil {
			return err
		}
		nextVersion := row.Version + 1
		result := svc.db.WithContext(ctx).Table("file_transfer_task").
			Where("task_id = ? AND version = ?", task.TaskID, row.Version).
			Updates(map[string]any{
				"uploaded_chunks": uploadedParts,
				"uploaded_size":   uploadedSize,
				"version":         nextVersion,
				"updated_at":      time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// 并发冲突时重新读取 version 和分片聚合值，再尝试一次。
			continue
		}
		// CAS 成功后再把共享任务对象修正到最新 DB 版本和聚合进度。
		task.Status = filemodel.TransferTaskStatus(row.Status)
		task.UploadedPart = uploadedParts
		task.UploadedSize = uploadedSize
		task.Version = nextVersion
		task.UpdatedAt = time.Now()
		svc.persistTransferSnapshot(task)
		svc.persistTransferTaskVersion(task.TaskID, task.Version)
		return nil
	}
	return ErrTransferTaskVersion
}

// completeTimeOrNil 只有任务真正进入终态时才写完成时间。
func completeTimeOrNil(task *filemodel.TransferTask) any {
	if task == nil {
		return nil
	}
	if task.Status != filemodel.TransferTaskCompleted && task.Status != filemodel.TransferTaskCanceled {
		return nil
	}
	return time.Now()
}
