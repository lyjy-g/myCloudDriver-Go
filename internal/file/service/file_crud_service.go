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
	// 首页信息只读内存视图，所以这里用读锁保护遍历过程。
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	var used int64
	recent := make([]filemodel.FileItem, 0)
	for _, it := range svc.items {
		if it.Deleted {
			// 已删除文件不应该进入首页统计和最近列表。
			continue
		}
		if !it.IsDir {
			// 已用空间只统计文件，不把目录本身算进容量。
			used += it.Size
		}
		recent = append(recent, *it)
	}
	// 最近列表按更新时间倒序返回，贴近用户首页“最近使用”的预期。
	sort.Slice(recent, func(i, j int) bool { return recent[i].UpdatedAt.After(recent[j].UpdatedAt) })
	if len(recent) > 10 {
		// 首页只保留有限条最近记录，避免一次返回整个工作区数据。
		recent = recent[:10]
	}
	return filemodel.HomeInfo{UsedBytes: used, Recent: recent}
}

// List 返回文件列表。
func (svc *FileService) List(ctx context.Context, parentID, keyword, settingID string) []filemodel.FileItem {
	if svc.db != nil {
		// 有 DB 时优先走 DB 查询，保证多实例和重启后的结果一致。
		items, err := svc.listFromDB(ctx, parentID, keyword, settingID)
		if err == nil {
			return items
		}
		// DB 查询失败时这里直接返回空列表，避免把不完整的内存态冒充成最终结果。
		return []filemodel.FileItem{}
	}
	// 无 DB 模式下回退到内存索引查询。
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	result := make([]filemodel.FileItem, 0)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for _, it := range svc.items {
		if it.Deleted {
			continue
		}
		if parentID != "" && it.ParentID != parentID {
			// 指定父目录时只返回当前目录下的直接子项。
			continue
		}
		if strings.TrimSpace(settingID) != "" && it.ParentID != "" && it.StorageSettingID != strings.TrimSpace(settingID) {
			// 根目录不强行按存储配置过滤，子目录和文件才按激活存储配置隔离视图。
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(it.Name), keyword) {
			// 关键字查询统一做大小写无关匹配，和 DB LIKE 的体验保持接近。
			continue
		}
		result = append(result, *it)
	}
	// 列表里目录优先，目录内再按名称排序，符合常见网盘展示习惯。
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
	// 目录列表直接复用通用列表查询，避免两套筛选逻辑漂移。
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
		// 多实例模式下优先读 DB，避免某个实例内存里还没同步到最新元数据。
		if item, err := svc.getFromDB(ctx, fileID); err == nil {
			return item, nil
		}
	}
	// 无 DB 或 DB 读失败时回退内存索引。
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	it, ok := svc.items[fileID]
	if !ok || it.Deleted {
		return nil, errors.New("file not found")
	}
	// 详情返回副本而不是原对象，避免上层误改共享内存状态。
	cp := *it
	return &cp, nil
}

// getFromDB 从 file_info 主表读取单个文件详情。
func (svc *FileService) getFromDB(ctx context.Context, fileID string) (*filemodel.FileItem, error) {
	// 先解析登录主体，确保文件详情始终按用户和 workspace 做隔离。
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	var row filedb.FileInfo
	// 这里只查未删除文件，回收站数据由单独接口负责。
	if err = svc.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND workspace_id = ? AND is_deleted = 0", fileID, p.UserID, p.WorkspaceID).
		First(&row).Error; err != nil {
		return nil, err
	}
	// DB 行统一映射回 FileItem DTO，供上层接口和业务复用。
	item := &filemodel.FileItem{
		ID:               row.ID,
		ParentID:         normalizeParentOutput(row.ParentID),
		StorageSettingID: row.StoragePlatformSettingID,
		Name:             row.DisplayName,
		IsDir:            row.IsDir,
		Size:             row.Size,
		FileHash:         strings.TrimSpace(row.ContentSha256),
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
		// 有 DB 时目录创建直接落库，避免不同实例目录树不一致。
		return svc.createDirectoryDB(ctx, parentID, name, settingID)
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if parentID == "" {
		// 缺省父目录统一映射到 root，减少内存态分叉判断。
		parentID = "root"
	}
	if _, ok := svc.items[parentID]; !ok {
		return nil, errors.New("parent not found")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		// 空目录名统一替换成默认值，保持接口行为稳定。
		name = "新建文件夹"
	}
	// 内存态目录创建也做自动重名处理，和 DB 路径口径一致。
	name = svc.uniqueNameLocked(parentID, name)

	now := time.Now()
	id := svc.nextIDLocked()
	it := &filemodel.FileItem{ID: id, ParentID: parentID, StorageSettingID: strings.TrimSpace(settingID), Name: name, IsDir: true, CreatedAt: now, UpdatedAt: now}
	// 创建完成后把目录挂进共享索引，后续列表/移动/路径查询都基于这里。
	svc.items[id] = it
	cp := *it
	return &cp, nil
}

// listFromDB 从 file_info 主表按目录、关键字和存储配置读取列表。
func (svc *FileService) listFromDB(ctx context.Context, parentID, keyword, settingID string) ([]filemodel.FileItem, error) {
	// DB 列表查询先基于登录主体限定用户和 workspace 范围。
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	parent := normalizeParentID(parentID)

	query := svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
		Where("user_id = ? AND workspace_id = ? AND is_deleted = 0", p.UserID, p.WorkspaceID)
	if parent == "" {
		// root 在 DB 里统一存空字符串，所以这里把各种 root 表达都并到同一条件里。
		query = query.Where("(parent_id = '' OR parent_id IS NULL OR parent_id = 'root' OR parent_id = 'ROOT')")
	} else {
		query = query.Where("parent_id = ?", parent)
	}
	if strings.TrimSpace(settingID) != "" {
		// 显式指定存储配置时，只返回该配置下的文件视图。
		query = query.Where("storage_platform_setting_id = ?", strings.TrimSpace(settingID))
	}
	if strings.TrimSpace(keyword) != "" {
		// DB 查询也做模糊匹配，尽量和内存态 contains 行为保持一致。
		kw := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("display_name LIKE ?", kw)
	}
	var rows []filedb.FileInfo
	if err = query.Order("is_dir desc, display_name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]filemodel.FileItem, 0, len(rows))
	for _, row := range rows {
		// 每一行都转成统一 DTO，避免上层直接依赖 DB model。
		items = append(items, filemodel.FileItem{
			ID:               row.ID,
			ParentID:         normalizeParentOutput(row.ParentID),
			StorageSettingID: row.StoragePlatformSettingID,
			Name:             row.DisplayName,
			IsDir:            row.IsDir,
			Size:             row.Size,
			FileHash:         strings.TrimSpace(row.ContentSha256),
			ObjectKey:        row.ObjectKey,
			CreatedAt:        row.UploadTime,
			UpdatedAt:        row.UpdateTime,
			Deleted:          row.IsDeleted,
		})
	}
	return items, nil
}

// createDirectoryDB 在 DB 模式下创建目录元数据。
func (svc *FileService) createDirectoryDB(ctx context.Context, parentID, name, settingID string) (*filemodel.FileItem, error) {
	// DB 模式下目录创建先拿主体信息，避免目录落错工作区。
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
	// 目录也复用 file_info 表，这样列表、移动、回收站都走同一套元数据结构。
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
		"content_sha256":              "",
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
	// 返回 DTO 副本，供接口层直接返回给前端。
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

// persistFileInfo 把最终文件元数据落到 file_info 表。
func (svc *FileService) persistFileInfo(ctx context.Context, item filemodel.FileItem) error {
	if svc.db == nil {
		return nil
	}
	// 文件元数据落库前先解析主体，确保 object_key 和元数据都绑定到当前用户/空间。
	p, err := security.RequireLogin(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	fileHash := strings.TrimSpace(item.FileHash)
	// 文件域现在统一只保存 SHA-256，避免多种 hash 混用带来歧义。
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
		"content_sha256":              fileHash,
		"storage_platform_setting_id": strings.TrimSpace(item.StorageSettingID),
		"upload_time":                 now,
		"update_time":                 now,
		"last_access_time":            now,
		"is_deleted":                  false,
		"deleted_time":                nil,
	}
	// 这里直接插入最终文件元数据；上传链路已经在上游保证 object 已经存在。
	return svc.db.WithContext(ctx).Table("file_info").Create(insert).Error
}

// normalizeParentID 把业务层的 root 表示规范化成 DB 存储格式。
func normalizeParentID(parentID string) string {
	switch strings.TrimSpace(parentID) {
	case "", "root", "ROOT":
		// DB 层统一把根目录表示成空字符串，避免 root/ROOT/空串多种写法混用。
		return ""
	default:
		return strings.TrimSpace(parentID)
	}
}

// normalizeParentOutput 把 DB 层的根目录表示还原成业务层使用的 root。
func normalizeParentOutput(parentID string) string {
	if strings.TrimSpace(parentID) == "" {
		// 输出给业务和前端时再把空字符串还原成 root，避免暴露底层存储细节。
		return "root"
	}
	return strings.TrimSpace(parentID)
}

// fileSuffix 提取文件名后缀并统一转成小写。
func fileSuffix(name string) string {
	name = strings.TrimSpace(name)
	idx := strings.LastIndex(name, ".")
	if idx <= 0 || idx >= len(name)-1 {
		// 没有合法后缀时统一返回空字符串，避免把隐藏文件前导点误判成后缀。
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
			// DB 模式下直接更新这条元数据，避免内存和 DB 各维护一套命名结果。
			if err = svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
				Where("id = ? AND user_id = ? AND workspace_id = ? AND is_deleted = 0", fileID, p.UserID, p.WorkspaceID).
				Updates(map[string]any{"display_name": newName, "original_name": newName, "update_time": now}).Error; err != nil {
				return nil, err
			}
			return svc.getFromDB(ctx, fileID)
		}
	}
	// 无 DB 模式下直接修改内存对象。
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
	// 重命名仍然做同目录内自动去重，避免生成重名项。
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
			// DB 模式下一次性批量更新 parent_id，减少多条 update 带来的窗口期。
			return svc.db.WithContext(ctx).Model(&filedb.FileInfo{}).
				Where("id IN ? AND user_id = ? AND workspace_id = ? AND is_deleted = 0", fileIDs, p.UserID, p.WorkspaceID).
				Updates(map[string]any{"parent_id": parent, "update_time": now}).Error
		}
	}
	// 无 DB 模式下用内存树做目录合法性校验和移动。
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
			// 目录不能移动到自己的子孙目录下，否则目录树会成环。
			return fmt.Errorf("cannot move dir into its child: %s", it.ID)
		}
		it.ParentID = targetParentID
		// 移动到新目录后也重新做一次重名处理，避免目标目录出现同名冲突。
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
			// 软删除只改删除标记，不触碰底层对象，方便后续恢复。
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
	// 无 DB 模式下同步更新内存删除标记和删除时间。
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
			// 恢复操作只撤销删除标记，不会改 object_key 等存储信息。
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
	// 无 DB 模式下恢复时顺手做一次重名处理，避免回收站恢复后覆盖现有文件名。
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

	// 先在元数据树里递归收集对象 key，再删除节点，避免删掉后找不到子项对象。
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
		// 对象删除逐个执行并累计报告，保证部分失败时也能知道失败对象有哪些。
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

	// 清空回收站时先删除所有已标记项的元数据，再统一删对象。
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
		// 和永久删除一样，回收站清空也按对象粒度记录删除失败结果。
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
			// DB 模式下回收站分页直接查已删除数据，避免把整个列表拉到内存后再分页。
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
					// DB model 里的零值时间统一转成 nil，避免前端拿到无意义的零时间。
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
					FileHash:         strings.TrimSpace(row.ContentSha256),
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
	// 无 DB 模式下在内存里做分页，逻辑上尽量贴近 DB 路径。
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
	// 回收站默认按最近删除/更新时间倒序展示。
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
	// 收藏状态是内存态字段，当前实现直接批量更新共享对象。
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
	// 路径查询只读目录树，所以这里用读锁即可。
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	cur, ok := svc.items[dirID]
	if !ok || cur.Deleted || !cur.IsDir {
		return nil, errors.New("dir not found")
	}
	pathItems := make([]filemodel.FileItem, 0)
	for cur != nil {
		// 先从当前目录一路向上回溯到 root。
		pathItems = append(pathItems, *cur)
		if cur.ParentID == "" {
			break
		}
		cur = svc.items[cur.ParentID]
	}
	for i, j := 0, len(pathItems)-1; i < j; i, j = i+1, j-1 {
		// 回溯得到的是倒序路径，这里再原地翻转成 root -> current。
		pathItems[i], pathItems[j] = pathItems[j], pathItems[i]
	}
	return pathItems, nil
}

// nextIDLocked 生成新的内存文件 ID。
func (svc *FileService) nextIDLocked() string {
	// 这里自增计数主要用于保留本地生成节奏，真正 ID 仍使用 UUID 降低碰撞概率。
	svc.counter++
	return "f_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

// uniqueNameLocked 在同一父目录内生成不冲突的名称。
func (svc *FileService) uniqueNameLocked(parentID, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unnamed"
	}
	base := name
	idx := 1
	// 同目录重名时按常见 "(n)" 规则自动递增生成可用名称。
	for svc.existsNameLocked(parentID, name) {
		name = fmt.Sprintf("%s(%d)", base, idx)
		idx++
	}
	return name
}

// existsNameLocked 判断同一父目录下是否已存在同名可见项。
func (svc *FileService) existsNameLocked(parentID, name string) bool {
	for _, it := range svc.items {
		if it.Deleted {
			continue
		}
		// 这里只检查同父目录下的可见项，已删除文件不参与重名判断。
		if it.ParentID == parentID && it.Name == name {
			return true
		}
	}
	return false
}

// isDescendantLocked 判断 candidate 是否位于 ancestor 子树中。
func (svc *FileService) isDescendantLocked(candidateID, ancestorID string) bool {
	if candidateID == ancestorID {
		// 自己也视为自己的后代检查命中，这样移动目录时能挡住“移到自己下面”。
		return true
	}
	cur := svc.items[candidateID]
	for cur != nil {
		if cur.ParentID == ancestorID {
			return true
		}
		if cur.ParentID == "" {
			// 走到根还没命中 ancestor，说明不在该子树里。
			return false
		}
		cur = svc.items[cur.ParentID]
	}
	return false
}

// deleteRecursiveLocked 递归删除内存目录树上的节点。
func (svc *FileService) deleteRecursiveLocked(id string) {
	// 先递归删子节点，再删自己，保证目录树删除时不会留下悬挂子项。
	for childID, it := range svc.items {
		if it.ParentID == id {
			svc.deleteRecursiveLocked(childID)
		}
	}
	delete(svc.items, id)
}

// collectDeleteTargetsLocked 递归统计要删除的元数据条数和对象 key。
func (svc *FileService) collectDeleteTargetsLocked(id string, objectKeys *[]string) int {
	it, ok := svc.items[id]
	if !ok {
		return 0
	}
	count := 1
	if !it.IsDir && strings.TrimSpace(it.ObjectKey) != "" {
		// 只有真实文件才需要删除底层对象，目录本身没有 object_key。
		*objectKeys = append(*objectKeys, it.ObjectKey)
	}
	for childID, child := range svc.items {
		if child.ParentID == id {
			count += svc.collectDeleteTargetsLocked(childID, objectKeys)
		}
	}
	return count
}

// dedupeStrings 去重字符串切片，保留首次出现顺序。
func dedupeStrings(items []string) []string {
	if len(items) <= 1 {
		return items
	}
	// 删除对象前先去重，避免目录树里多次收集到同一个 object_key 时重复删对象。
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
