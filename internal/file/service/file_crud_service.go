package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	filemodel "myclouddrive-go/internal/file/model"
	filedb "myclouddrive-go/internal/file/model/dbmodel"
	"myclouddrive-go/internal/framework/security"
)

// Home 返回首页信息。
func (svc *FileService) Home(_ context.Context) filemodel.HomeInfo {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	var used int64
	recent := make([]filemodel.FileItem, 0)
	for _, it := range svc.items {
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
func (svc *FileService) List(ctx context.Context, parentID, keyword, settingID string) []filemodel.FileItem {
	if svc.db != nil {
		items, err := svc.listFromDB(ctx, parentID, keyword, settingID)
		if err == nil {
			return items
		}
		return []filemodel.FileItem{}
	}
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	result := make([]filemodel.FileItem, 0)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for _, it := range svc.items {
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
func (svc *FileService) ListDirs(ctx context.Context, parentID, settingID string) []filemodel.FileItem {
	items := svc.List(ctx, parentID, "", settingID)
	result := make([]filemodel.FileItem, 0, len(items))
	for _, it := range items {
		if it.IsDir {
			result = append(result, it)
		}
	}
	return result
}

// Get 读取文件详情。
func (svc *FileService) Get(ctx context.Context, fileID string) (*filemodel.FileItem, error) {
	if svc.db != nil {
		if item, err := svc.getFromDB(ctx, fileID); err == nil {
			return item, nil
		}
	}
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	it, ok := svc.items[fileID]
	if !ok || it.Deleted {
		return nil, errors.New("file not found")
	}
	cp := *it
	return &cp, nil
}

func (svc *FileService) getFromDB(ctx context.Context, fileID string) (*filemodel.FileItem, error) {
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	var row filedb.FileInfo
	if err = svc.db.WithContext(ctx).
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
func (svc *FileService) CreateDirectory(ctx context.Context, parentID, name, settingID string) (*filemodel.FileItem, error) {
	if svc.db != nil {
		return svc.createDirectoryDB(ctx, parentID, name, settingID)
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if parentID == "" {
		parentID = "root"
	}
	if _, ok := svc.items[parentID]; !ok {
		return nil, errors.New("parent not found")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "新建文件夹"
	}
	name = svc.uniqueNameLocked(parentID, name)

	now := time.Now()
	id := svc.nextIDLocked()
	it := &filemodel.FileItem{ID: id, ParentID: parentID, StorageSettingID: strings.TrimSpace(settingID), Name: name, IsDir: true, CreatedAt: now, UpdatedAt: now}
	svc.items[id] = it
	cp := *it
	return &cp, nil
}

func (svc *FileService) listFromDB(ctx context.Context, parentID, keyword, settingID string) ([]filemodel.FileItem, error) {
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	parent := normalizeParentID(parentID)

	query := svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
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

func (svc *FileService) createDirectoryDB(ctx context.Context, parentID, name, settingID string) (*filemodel.FileItem, error) {
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
	if err = svc.db.WithContext(ctx).Table("file_info").Create(insert).Error; err != nil {
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

func (svc *FileService) persistFileInfo(ctx context.Context, item filemodel.FileItem) error {
	if svc.db == nil {
		return nil
	}
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	fileHash := strings.TrimSpace(item.FileHash)
	if len(fileHash) > 32 {
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
	return svc.db.WithContext(ctx).Table("file_info").Create(insert).Error
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
func (svc *FileService) Rename(fileID, newName string) (*filemodel.FileItem, error) {
	return svc.RenameWithContext(context.Background(), fileID, newName)
}

// RenameWithContext 重命名（DB 优先）。
func (svc *FileService) RenameWithContext(ctx context.Context, fileID, newName string) (*filemodel.FileItem, error) {
	if svc.db != nil {
		p, err := security.RequireLogin(ctx)
		if err == nil {
			newName = strings.TrimSpace(newName)
			if newName == "" {
				return nil, errors.New("name required")
			}
			now := time.Now()
			if err = svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
				Where("id = ? AND user_id = ? AND workspace_id = ? AND is_deleted = 0", fileID, p.UserID, p.WorkspaceID).
				Updates(map[string]any{"display_name": newName, "original_name": newName, "update_time": now}).Error; err != nil {
				return nil, err
			}
			return svc.getFromDB(ctx, fileID)
		}
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()

	it, ok := svc.items[fileID]
	if !ok || it.Deleted {
		return nil, errors.New("file not found")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, errors.New("name required")
	}
	it.Name = svc.uniqueNameLocked(it.ParentID, newName)
	it.UpdatedAt = time.Now()
	cp := *it
	return &cp, nil
}

// Move 移动文件。
func (svc *FileService) Move(fileIDs []string, targetParentID string) error {
	return svc.MoveWithContext(context.Background(), fileIDs, targetParentID)
}

// MoveWithContext 移动文件（DB 优先）。
func (svc *FileService) MoveWithContext(ctx context.Context, fileIDs []string, targetParentID string) error {
	if svc.db != nil {
		p, err := security.RequireLogin(ctx)
		if err == nil {
			parent := normalizeParentID(targetParentID)
			now := time.Now()
			return svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
				Where("id IN ? AND user_id = ? AND workspace_id = ? AND is_deleted = 0", fileIDs, p.UserID, p.WorkspaceID).
				Updates(map[string]any{"parent_id": parent, "update_time": now}).Error
		}
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()

	parent, ok := svc.items[targetParentID]
	if !ok || parent.Deleted || !parent.IsDir {
		return errors.New("target parent not found")
	}
	for _, id := range fileIDs {
		it, exists := svc.items[id]
		if !exists || it.Deleted {
			continue
		}
		if it.IsDir && svc.isDescendantLocked(targetParentID, it.ID) {
			return fmt.Errorf("cannot move dir into its child: %s", it.ID)
		}
		it.ParentID = targetParentID
		it.Name = svc.uniqueNameLocked(targetParentID, it.Name)
		it.UpdatedAt = time.Now()
	}
	return nil
}

// Recycle 软删除。
func (svc *FileService) Recycle(ctx context.Context, fileIDs []string) {
	if svc.db != nil {
		p, err := security.RequireLogin(ctx)
		if err == nil && len(fileIDs) > 0 {
			now := time.Now()
			_ = svc.db.WithContext(ctx).
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
	svc.mu.Lock()
	defer svc.mu.Unlock()
	now := time.Now()
	for _, id := range fileIDs {
		if it, ok := svc.items[id]; ok && !it.Deleted {
			it.Deleted = true
			it.DeletedAt = &now
			it.UpdatedAt = now
		}
	}
}

// Restore 从回收站恢复。
func (svc *FileService) Restore(ctx context.Context, fileIDs []string) {
	if svc.db != nil {
		p, err := security.RequireLogin(ctx)
		if err == nil && len(fileIDs) > 0 {
			now := time.Now()
			_ = svc.db.WithContext(ctx).
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
	svc.mu.Lock()
	defer svc.mu.Unlock()
	for _, id := range fileIDs {
		if it, ok := svc.items[id]; ok && it.Deleted {
			it.Deleted = false
			it.DeletedAt = nil
			it.UpdatedAt = time.Now()
			it.Name = svc.uniqueNameLocked(it.ParentID, it.Name)
		}
	}
}

// PermanentlyDelete 永久删除（元数据 + 插件对象删除）。
func (svc *FileService) PermanentlyDelete(ctx context.Context, fileIDs []string) filemodel.HardDeleteReport {
	report := filemodel.HardDeleteReport{
		Requested:        len(fileIDs),
		FailedObjectKeys: make([]string, 0),
	}

	svc.mu.Lock()
	objectKeys := make([]string, 0)
	for _, id := range fileIDs {
		report.MetadataDeleted += svc.collectDeleteTargetsLocked(id, &objectKeys)
		svc.deleteRecursiveLocked(id)
	}
	svc.mu.Unlock()

	if svc.storage == nil || len(objectKeys) == 0 {
		return report
	}

	for _, key := range dedupeStrings(objectKeys) {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if err := svc.storage.Delete(ctx, key); err != nil {
			report.ObjectDeleteFailed++
			report.FailedObjectKeys = append(report.FailedObjectKeys, key)
			continue
		}
		report.ObjectDeleteSuccess++
	}
	return report
}

// ClearRecycle 清空回收站（元数据 + 插件对象删除）。
func (svc *FileService) ClearRecycle(ctx context.Context) filemodel.HardDeleteReport {
	report := filemodel.HardDeleteReport{FailedObjectKeys: make([]string, 0)}

	svc.mu.Lock()
	objectKeys := make([]string, 0)
	for id, it := range svc.items {
		if it.Deleted {
			report.Requested++
			report.MetadataDeleted += svc.collectDeleteTargetsLocked(id, &objectKeys)
			delete(svc.items, id)
		}
	}
	svc.mu.Unlock()

	if svc.storage == nil || len(objectKeys) == 0 {
		return report
	}
	for _, key := range dedupeStrings(objectKeys) {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if err := svc.storage.Delete(ctx, key); err != nil {
			report.ObjectDeleteFailed++
			report.FailedObjectKeys = append(report.FailedObjectKeys, key)
			continue
		}
		report.ObjectDeleteSuccess++
	}
	return report
}

// ListRecycle 分页返回回收站。
func (svc *FileService) ListRecycle(ctx context.Context, page, size int) ([]filemodel.FileItem, int) {
	if svc.db != nil {
		p, err := security.RequireLogin(ctx)
		if err == nil {
			if page <= 0 {
				page = 1
			}
			if size <= 0 {
				size = 20
			}
			query := svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
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
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	items := make([]filemodel.FileItem, 0)
	for _, it := range svc.items {
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
func (svc *FileService) SetFavorite(fileIDs []string, favorite bool) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	now := time.Now()
	for _, id := range fileIDs {
		if it, ok := svc.items[id]; ok && !it.Deleted {
			it.Favorite = favorite
			it.UpdatedAt = now
		}
	}
}

// DirPath 返回目录层级路径。
func (svc *FileService) DirPath(dirID string) ([]filemodel.FileItem, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	cur, ok := svc.items[dirID]
	if !ok || cur.Deleted || !cur.IsDir {
		return nil, errors.New("dir not found")
	}
	pathItems := make([]filemodel.FileItem, 0)
	for cur != nil {
		pathItems = append(pathItems, *cur)
		if cur.ParentID == "" {
			break
		}
		cur = svc.items[cur.ParentID]
	}
	for i, j := 0, len(pathItems)-1; i < j; i, j = i+1, j-1 {
		pathItems[i], pathItems[j] = pathItems[j], pathItems[i]
	}
	return pathItems, nil
}

func (svc *FileService) nextIDLocked() string {
	svc.counter++
	return "f_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func (svc *FileService) uniqueNameLocked(parentID, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unnamed"
	}
	base := name
	idx := 1
	for svc.existsNameLocked(parentID, name) {
		name = fmt.Sprintf("%s(%d)", base, idx)
		idx++
	}
	return name
}

func (svc *FileService) existsNameLocked(parentID, name string) bool {
	for _, it := range svc.items {
		if it.Deleted {
			continue
		}
		if it.ParentID == parentID && it.Name == name {
			return true
		}
	}
	return false
}

func (svc *FileService) isDescendantLocked(candidateID, ancestorID string) bool {
	if candidateID == ancestorID {
		return true
	}
	cur := svc.items[candidateID]
	for cur != nil {
		if cur.ParentID == ancestorID {
			return true
		}
		if cur.ParentID == "" {
			return false
		}
		cur = svc.items[cur.ParentID]
	}
	return false
}

func (svc *FileService) deleteRecursiveLocked(id string) {
	for childID, it := range svc.items {
		if it.ParentID == id {
			svc.deleteRecursiveLocked(childID)
		}
	}
	delete(svc.items, id)
}

func (svc *FileService) collectDeleteTargetsLocked(id string, objectKeys *[]string) int {
	it, ok := svc.items[id]
	if !ok {
		return 0
	}
	count := 1
	if !it.IsDir && strings.TrimSpace(it.ObjectKey) != "" {
		*objectKeys = append(*objectKeys, it.ObjectKey)
	}
	for childID, child := range svc.items {
		if child.ParentID == id {
			count += svc.collectDeleteTargetsLocked(childID, objectKeys)
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
