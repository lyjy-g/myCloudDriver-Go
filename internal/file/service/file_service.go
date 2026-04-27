package service

import (
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

	storagemodel "myclouddrive-go/internal/storage/model"
	storagesvc "myclouddrive-go/internal/storage/service"
)

// FileItem 表示文件元数据（内存实现）。
type FileItem struct {
	ID        string     `json:"id"`
	ParentID  string     `json:"parent_id"`
	Name      string     `json:"name"`
	IsDir     bool       `json:"is_dir"`
	Size      int64      `json:"size"`
	ObjectKey string     `json:"object_key,omitempty"`
	Favorite  bool       `json:"favorite"`
	Deleted   bool       `json:"deleted"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// HomeInfo 表示文件首页信息。
type HomeInfo struct {
	UsedBytes int64      `json:"used_bytes"`
	Recent    []FileItem `json:"recent"`
}

// StorageGateway 定义文件模块依赖的最小存储能力集合。
type StorageGateway interface {
	PresignDownloadURL(ctx context.Context, key string, expire time.Duration) (string, error)
	Get(ctx context.Context, key string) (io.ReadCloser, storagemodel.ObjectInfo, error)
	Delete(ctx context.Context, key string) error
}

// HardDeleteReport 记录“元数据 + 对象存储”双写删除结果。
type HardDeleteReport struct {
	Requested           int      `json:"requested"`
	MetadataDeleted     int      `json:"metadataDeleted"`
	ObjectDeleteSuccess int      `json:"objectDeleteSuccess"`
	ObjectDeleteFailed  int      `json:"objectDeleteFailed"`
	FailedObjectKeys    []string `json:"failedObjectKeys,omitempty"`
}

// FileService 是 file 模块的唯一实现。
type FileService struct {
	mu      sync.RWMutex
	counter int64
	items   map[string]*FileItem
	storage StorageGateway

	idemMu      sync.Mutex
	idemRecords map[string]idempotencyRecord
	idemTTL     time.Duration
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
func NewFileService(storage *storagesvc.StorageService) *FileService {
	now := time.Now()
	root := &FileItem{
		ID:        "root",
		ParentID:  "",
		Name:      "/",
		IsDir:     true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	doc := &FileItem{
		ID:        "f_1",
		ParentID:  "root",
		Name:      "readme.txt",
		IsDir:     false,
		Size:      128,
		ObjectKey: "demo/readme.txt",
		CreatedAt: now,
		UpdatedAt: now,
	}
	return &FileService{
		counter:     2,
		items:       map[string]*FileItem{root.ID: root, doc.ID: doc},
		storage:     storage,
		idemRecords: make(map[string]idempotencyRecord),
		idemTTL:     24 * time.Hour,
	}
}

// Ping 服务健康检查。
func (s *FileService) Ping(_ context.Context) (string, error) {
	return "file service ready", nil
}

// Home 返回首页信息。
func (s *FileService) Home(_ context.Context) HomeInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var used int64
	recent := make([]FileItem, 0)
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
	return HomeInfo{UsedBytes: used, Recent: recent}
}

// List 返回文件列表。
func (s *FileService) List(parentID, keyword string) []FileItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]FileItem, 0)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for _, it := range s.items {
		if it.Deleted {
			continue
		}
		if parentID != "" && it.ParentID != parentID {
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
func (s *FileService) ListDirs(parentID string) []FileItem {
	items := s.List(parentID, "")
	result := make([]FileItem, 0, len(items))
	for _, it := range items {
		if it.IsDir {
			result = append(result, it)
		}
	}
	return result
}

// Get 读取文件详情。
func (s *FileService) Get(fileID string) (*FileItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[fileID]
	if !ok || it.Deleted {
		return nil, errors.New("file not found")
	}
	cp := *it
	return &cp, nil
}

// CreateDirectory 创建目录（自动重名处理）。
func (s *FileService) CreateDirectory(parentID, name string) (*FileItem, error) {
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
	it := &FileItem{ID: id, ParentID: parentID, Name: name, IsDir: true, CreatedAt: now, UpdatedAt: now}
	s.items[id] = it
	cp := *it
	return &cp, nil
}

// Rename 重命名。
func (s *FileService) Rename(fileID, newName string) (*FileItem, error) {
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
func (s *FileService) Recycle(fileIDs []string) {
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
func (s *FileService) Restore(fileIDs []string) {
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
func (s *FileService) PermanentlyDelete(ctx context.Context, fileIDs []string) HardDeleteReport {
	report := HardDeleteReport{
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
func (s *FileService) ClearRecycle(ctx context.Context) HardDeleteReport {
	report := HardDeleteReport{FailedObjectKeys: make([]string, 0)}

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
func (s *FileService) ListRecycle(page, size int) ([]FileItem, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	items := make([]FileItem, 0)
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
		return []FileItem{}, total
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
func (s *FileService) DirPath(dirID string) ([]FileItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cur, ok := s.items[dirID]
	if !ok || cur.Deleted || !cur.IsDir {
		return nil, errors.New("dir not found")
	}
	pathItems := make([]FileItem, 0)
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
	return fmt.Sprintf("f_%d", s.counter)
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
func (s *FileService) ResolveDownloadURL(ctx context.Context, fileID string, expire time.Duration) (string, *FileItem, error) {
	item, err := s.Get(fileID)
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
func (s *FileService) OpenPreviewContent(ctx context.Context, fileID string) (io.ReadCloser, storagemodel.ObjectInfo, *FileItem, error) {
	item, err := s.Get(fileID)
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
