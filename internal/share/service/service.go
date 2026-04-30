package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	filedb "myclouddrive-go/internal/file/model/dbmodel"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	sharedto "myclouddrive-go/internal/share/model"
	sharedb "myclouddrive-go/internal/share/model/dbmodel"
	storagemodel "myclouddrive-go/internal/storage/model"
)

// ShareService 是分享模块唯一实现。
type ShareService struct {
	db      *gorm.DB
	storage StorageGateway
}

// DTO 类型别名：保持 service/api 层调用方式不变，DTO 统一收敛到 share/model。
type (
	ShareVO               = sharedto.ShareVO
	ShareFileVO           = sharedto.ShareFileVO
	CreateShareReq        = sharedto.CreateShareReq
	UpdateShareReq        = sharedto.UpdateShareReq
	VerifyShareCodeReq    = sharedto.VerifyShareCodeReq
	FileShareAccessRecord = sharedb.ShareAccessRecord
)

type shareRecord struct {
	ID               string
	UserID           string
	WorkspaceID      string
	StorageSettingID string
	ShareName        string
	ShareCode        string
	ExpireTime       *time.Time
	Scope            string
	ViewCount        int
	MaxViewCount     *int
	DownloadCount    int
	MaxDownloadCount *int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// StorageGateway 定义分享模块下载文件所需的最小存储能力。
type StorageGateway interface {
	Get(ctx context.Context, key string) (io.ReadCloser, storagemodel.ObjectInfo, error)
	GetBySetting(ctx context.Context, settingID string, key string) (io.ReadCloser, storagemodel.ObjectInfo, error)
}

func NewService(db *gorm.DB, storage StorageGateway) *ShareService {
	return &ShareService{db: db, storage: storage}
}

func (s *ShareService) Ping(_ context.Context) (string, error) {
	return "share service ready", nil
}

func currentUserID(ctx context.Context) (string, error) {
	p, ok := security.GetCtxInfo(ctx)
	if !ok || strings.TrimSpace(p.UserID) == "" {
		return "", code.New(code.NoPermission, "login required")
	}
	return p.UserID, nil
}

func toShareVO(row shareRecord, fileIDs []string) ShareVO {
	allowDownload := strings.Contains(strings.ToLower(row.Scope), "download")
	return ShareVO{
		ShareID:       row.ID,
		ShareName:     row.ShareName,
		ShareCode:     row.ShareCode,
		WorkspaceID:   row.WorkspaceID,
		SettingID:     row.StorageSettingID,
		AllowDownload: allowDownload,
		ExpireTime:    row.ExpireTime,
		ViewCount:     row.ViewCount,
		DownloadCount: row.DownloadCount,
		Status:        statusFromShare(row),
		FileIDs:       fileIDs,
		AccessPath:    "/share/" + row.ID,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func statusFromShare(share shareRecord) int {
	now := time.Now()
	if share.ExpireTime != nil && now.After(*share.ExpireTime) {
		return 1
	}
	if share.MaxViewCount != nil && share.ViewCount >= *share.MaxViewCount {
		return 1
	}
	if share.MaxDownloadCount != nil && share.DownloadCount >= *share.MaxDownloadCount {
		return 1
	}
	return 0
}

func randomID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func randomShareCode(n int) string {
	if n <= 0 {
		n = 6
	}
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(buf)
}

func normalizeScope(allowDownload bool) string {
	if allowDownload {
		return "preview,download"
	}
	return "preview"
}

func (s *ShareService) CreateShare(ctx context.Context, req CreateShareReq) (*ShareVO, error) {
	principal, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(principal.UserID)
	workspaceID := strings.TrimSpace(principal.WorkspaceID)
	if workspaceID == "" {
		return nil, code.New(code.BadRequest, "workspace is required")
	}
	if len(req.FileIDs) == 0 {
		return nil, code.New(code.BadRequest, "fileIds is required")
	}

	fileRows := make([]filedb.FileInfo, 0)
	if err = s.db.WithContext(ctx).
		Where("id IN ? AND is_deleted = 0 AND workspace_id = ?", req.FileIDs, workspaceID).
		Find(&fileRows).Error; err != nil {
		return nil, fmt.Errorf("query files failed: %w", err)
	}
	if len(fileRows) == 0 {
		return nil, code.New(code.BadRequest, "shared files not found")
	}
	storageSettingID := strings.TrimSpace(fileRows[0].StoragePlatformSettingID)
	for _, row := range fileRows {
		if strings.TrimSpace(row.WorkspaceID) != workspaceID {
			return nil, code.New(code.BadRequest, "files must belong to current workspace")
		}
		if strings.TrimSpace(row.StoragePlatformSettingID) != storageSettingID {
			return nil, code.New(code.BadRequest, "files in one share must use same storage setting")
		}
	}

	shareName := strings.TrimSpace(req.ShareName)
	if shareName == "" {
		shareName = fileRows[0].DisplayName
		if len(fileRows) > 1 {
			shareName = fmt.Sprintf("%s 等%d个文件", fileRows[0].DisplayName, len(fileRows))
		}
	}

	shareCode := strings.TrimSpace(req.ShareCode)
	if shareCode == "" {
		shareCode = randomShareCode(6)
	}
	allowDownload := true
	if req.AllowDownload != nil {
		allowDownload = *req.AllowDownload
	}
	now := time.Now()
	var expireAt *time.Time
	if req.ExpireSeconds != nil && *req.ExpireSeconds > 0 {
		t := now.Add(time.Duration(*req.ExpireSeconds) * time.Second)
		expireAt = &t
	}

	id := randomID("shr")
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		shareInsert := map[string]any{
			"id":                 id,
			"user_id":            userID,
			"workspace_id":       workspaceID,
			"storage_setting_id": storageSettingID,
			"share_name":         shareName,
			"share_code":         shareCode,
			"expire_time":        expireAt,
			"scope":              normalizeScope(allowDownload),
			"view_count":         0,
			"download_count":     0,
			"created_at":         now,
			"updated_at":         now,
		}
		if errTx := tx.Table("share_info").Create(shareInsert).Error; errTx != nil {
			return errTx
		}
		items := make([]sharedb.ShareItem, 0, len(req.FileIDs))
		for _, fid := range req.FileIDs {
			items = append(items, sharedb.ShareItem{ShareID: id, FileID: fid, CreatedAt: now})
		}
		if errTx := tx.Create(&items).Error; errTx != nil {
			return errTx
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create share failed: %w", err)
	}

	vo := toShareVO(shareRecord{
		ID:               id,
		UserID:           userID,
		WorkspaceID:      workspaceID,
		StorageSettingID: storageSettingID,
		ShareName:        shareName,
		ShareCode:        shareCode,
		ExpireTime:       expireAt,
		Scope:            normalizeScope(allowDownload),
		ViewCount:        0,
		DownloadCount:    0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, req.FileIDs)
	s.fillShareScopeNames(ctx, &vo)
	return &vo, nil
}

func (s *ShareService) ListMyShares(ctx context.Context) ([]ShareVO, error) {
	principal, err := security.RequireLogin(ctx)
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(principal.UserID)
	workspaceID := strings.TrimSpace(principal.WorkspaceID)
	rows := make([]shareRecord, 0)
	if err = s.db.WithContext(ctx).
		Table("share_info").
		Select("id, user_id, workspace_id, storage_setting_id, share_name, share_code, expire_time, scope, view_count, max_view_count, download_count, max_download_count, created_at, updated_at").
		Where("user_id = ? AND workspace_id = ?", userID, workspaceID).
		Order("created_at desc").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query shares failed: %w", err)
	}

	result := make([]ShareVO, 0, len(rows))
	for _, row := range rows {
		ids, _ := s.loadShareFileIDs(ctx, row.ID)
		result = append(result, toShareVO(row, ids))
	}
	s.fillShareScopeNamesBatch(ctx, result)
	return result, nil
}

func (s *ShareService) GetShareDetail(ctx context.Context, shareID string, requireOwner bool) (*ShareVO, error) {
	row, err := s.getShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	if requireOwner {
		uid, uidErr := currentUserID(ctx)
		if uidErr != nil {
			return nil, uidErr
		}
		if row.UserID != uid {
			return nil, code.New(code.NoPermission, "share not owned by current user")
		}
	}
	ids, _ := s.loadShareFileIDs(ctx, shareID)
	vo := toShareVO(*row, ids)
	s.fillShareScopeNames(ctx, &vo)
	return &vo, nil
}

func (s *ShareService) UpdateShare(ctx context.Context, shareID string, req UpdateShareReq) (*ShareVO, error) {
	uid, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	row, err := s.getShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	if row.UserID != uid {
		return nil, code.New(code.NoPermission, "share not owned by current user")
	}

	updates := map[string]any{"updated_at": time.Now()}
	if strings.TrimSpace(req.ShareName) != "" {
		updates["share_name"] = strings.TrimSpace(req.ShareName)
	}
	if req.AllowDownload != nil {
		updates["scope"] = normalizeScope(*req.AllowDownload)
	}
	if req.ExpireSeconds != nil {
		if *req.ExpireSeconds <= 0 {
			updates["expire_time"] = nil
		} else {
			t := time.Now().Add(time.Duration(*req.ExpireSeconds) * time.Second)
			updates["expire_time"] = t
		}
	}
	if req.ShareCode != "" {
		codeValue := strings.TrimSpace(req.ShareCode)
		if codeValue == "" {
			codeValue = randomShareCode(6)
		}
		updates["share_code"] = codeValue
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if errTx := tx.Table("share_info").Where("id = ?", shareID).Updates(updates).Error; errTx != nil {
			return errTx
		}
		if len(req.FileIDs) > 0 {
			if errTx := tx.Where("share_id = ?", shareID).Delete(&sharedb.ShareItem{}).Error; errTx != nil {
				return errTx
			}
			items := make([]sharedb.ShareItem, 0, len(req.FileIDs))
			now := time.Now()
			for _, fid := range req.FileIDs {
				items = append(items, sharedb.ShareItem{ShareID: shareID, FileID: fid, CreatedAt: now})
			}
			if errTx := tx.Create(&items).Error; errTx != nil {
				return errTx
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update share failed: %w", err)
	}
	return s.GetShareDetail(ctx, shareID, true)
}

func (s *ShareService) VerifyShareCode(ctx context.Context, shareID, shareCode string) (bool, error) {
	time.Sleep(200 * time.Millisecond)
	row, err := s.getShare(ctx, shareID)
	if err != nil {
		return false, err
	}
	if statusFromShare(*row) != 0 {
		return false, code.New(code.BadRequest, "share expired or exhausted")
	}
	expect := strings.TrimSpace(row.ShareCode)
	if expect != "" && !strings.EqualFold(expect, strings.TrimSpace(shareCode)) {
		return false, code.New(code.BadRequest, "invalid share code")
	}
	return true, nil
}

func (s *ShareService) PublicAccess(ctx context.Context, shareID, shareCode string, r *http.Request) (*ShareVO, error) {
	ok, err := s.VerifyShareCode(ctx, shareID, shareCode)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, code.New(code.BadRequest, "share code verify failed")
	}
	files, err := s.GetShareItems(ctx, shareID, "")
	if err != nil {
		return nil, err
	}
	row, err := s.getShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	_ = s.db.WithContext(ctx).Table("share_info").Where("id = ?", shareID).Update("view_count", gorm.Expr("view_count + 1")).Error
	_ = s.createAccessRecord(ctx, shareID, r)

	ids := make([]string, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.FileID)
	}
	vo := toShareVO(*row, ids)
	s.fillShareScopeNames(ctx, &vo)
	vo.Files = files
	return &vo, nil
}

func (s *ShareService) GetShareInfo(ctx context.Context, shareID string) (*ShareVO, error) {
	row, err := s.getShare(ctx, shareID)
	if err != nil {
		return nil, err
	}
	ids, _ := s.loadShareFileIDs(ctx, shareID)
	vo := toShareVO(*row, ids)
	s.fillShareScopeNames(ctx, &vo)
	return &vo, nil
}

func (s *ShareService) GetShareItems(ctx context.Context, shareID, parentID string) ([]ShareFileVO, error) {
	if _, err := s.getShare(ctx, shareID); err != nil {
		return nil, err
	}
	fileIDs, err := s.loadShareFileIDs(ctx, shareID)
	if err != nil {
		return nil, err
	}
	if len(fileIDs) == 0 {
		return []ShareFileVO{}, nil
	}
	rows := make([]filedb.FileInfo, 0)
	q := s.db.WithContext(ctx).Where("id IN ?", fileIDs)
	if strings.TrimSpace(parentID) != "" {
		q = q.Where("parent_id = ?", parentID)
	}
	if err = q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query shared files failed: %w", err)
	}
	items := make([]ShareFileVO, 0, len(rows))
	for _, row := range rows {
		item := ShareFileVO{FileID: row.ID, FileName: row.DisplayName, FileSize: row.Size, Directory: row.IsDir}
		if !row.IsDir {
			item.DownloadURL = "/apis/shares/public/" + shareID + "/download/" + row.ID
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *ShareService) DownloadShareFile(ctx context.Context, shareID, fileID, shareCode string, r *http.Request) (io.ReadCloser, storagemodel.ObjectInfo, string, error) {
	ok, err := s.VerifyShareCode(ctx, shareID, shareCode)
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, "", err
	}
	if !ok {
		return nil, storagemodel.ObjectInfo{}, "", code.New(code.BadRequest, "share code verify failed")
	}
	count, err := s.isFileInShare(ctx, shareID, fileID)
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, "", err
	}
	if count == 0 {
		return nil, storagemodel.ObjectInfo{}, "", code.New(code.NoPermission, "file not in share")
	}
	shareRow, err := s.getShare(ctx, shareID)
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, "", err
	}

	var file filedb.FileInfo
	if err = s.db.WithContext(ctx).Where("id = ?", fileID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, storagemodel.ObjectInfo{}, "", code.New(code.NotFound, "file not found")
		}
		return nil, storagemodel.ObjectInfo{}, "", fmt.Errorf("query file failed: %w", err)
	}
	if file.IsDir {
		return nil, storagemodel.ObjectInfo{}, "", code.New(code.BadRequest, "directory cannot be downloaded")
	}
	if file.IsDeleted {
		return nil, storagemodel.ObjectInfo{}, "", code.New(code.NotFound, "file not found")
	}
	if strings.TrimSpace(file.ObjectKey) == "" {
		return nil, storagemodel.ObjectInfo{}, "", code.New(code.NotFound, "file object not found")
	}
	if s.storage == nil {
		return nil, storagemodel.ObjectInfo{}, "", code.New(code.InternalError, "storage service unavailable")
	}

	// 先读取对象成功再累加计数，保证“计数推进”和“真实可下载”一致。
	targetSettingID := strings.TrimSpace(shareRow.StorageSettingID)
	if targetSettingID == "" {
		targetSettingID = strings.TrimSpace(file.StoragePlatformSettingID)
	}
	if targetSettingID == "" {
		return nil, storagemodel.ObjectInfo{}, "", code.New(code.BadRequest, "share storage setting is empty")
	}
	// 关键：分享下载必须按“分享归属配置”读取，避免切换激活配置后读错存储实例。
	rc, info, err := s.storage.GetBySetting(ctx, targetSettingID, strings.TrimSpace(file.ObjectKey))
	if err != nil {
		return nil, storagemodel.ObjectInfo{}, "", fmt.Errorf("open shared object failed: %w", err)
	}
	_ = s.db.WithContext(ctx).Table("share_info").Where("id = ?", shareID).Update("download_count", gorm.Expr("download_count + 1")).Error
	_ = s.createAccessRecord(ctx, shareID, r)
	return rc, info, file.DisplayName, nil
}

func (s *ShareService) ListAccessRecords(ctx context.Context, shareID string) ([]FileShareAccessRecord, error) {
	rows := make([]FileShareAccessRecord, 0)
	if err := s.db.WithContext(ctx).Where("share_id = ?", shareID).Order("access_time desc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query access records failed: %w", err)
	}
	return rows, nil
}

func (s *ShareService) CancelShares(ctx context.Context, shareIDs []string) error {
	uid, err := currentUserID(ctx)
	if err != nil {
		return err
	}
	if len(shareIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if errTx := tx.Where("share_id IN ?", shareIDs).Delete(&sharedb.ShareItem{}).Error; errTx != nil {
			return errTx
		}
		if errTx := tx.Where("id IN ? AND user_id = ?", shareIDs, uid).Delete(&sharedb.ShareInfo{}).Error; errTx != nil {
			return errTx
		}
		return nil
	})
}

func (s *ShareService) CancelAllShares(ctx context.Context) error {
	uid, err := currentUserID(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0)
	if err = s.db.WithContext(ctx).Model(&sharedb.ShareInfo{}).Where("user_id = ?", uid).Pluck("id", &ids).Error; err != nil {
		return fmt.Errorf("query user shares failed: %w", err)
	}
	return s.CancelShares(ctx, ids)
}

func (s *ShareService) getShare(ctx context.Context, shareID string) (*shareRecord, error) {
	row := &shareRecord{}
	query := s.db.WithContext(ctx).Table("share_info").
		Select("id, user_id, workspace_id, storage_setting_id, share_name, share_code, expire_time, scope, view_count, max_view_count, download_count, max_download_count, created_at, updated_at").
		Where("id = ?", shareID)
	if principal, ok := security.GetCtxInfo(ctx); ok && strings.TrimSpace(principal.WorkspaceID) != "" {
		query = query.Where("workspace_id = ?", strings.TrimSpace(principal.WorkspaceID))
	}
	if err := query.First(row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, code.New(code.NotFound, "share not found")
		}
		return nil, fmt.Errorf("query share failed: %w", err)
	}
	return row, nil
}

func (s *ShareService) fillShareScopeNames(ctx context.Context, vo *ShareVO) {
	if vo == nil {
		return
	}
	items := []ShareVO{*vo}
	s.fillShareScopeNamesBatch(ctx, items)
	*vo = items[0]
}

func (s *ShareService) fillShareScopeNamesBatch(ctx context.Context, items []ShareVO) {
	if len(items) == 0 {
		return
	}
	workspaceIDs := make([]string, 0)
	settingIDs := make([]string, 0)
	workspaceSeen := make(map[string]struct{})
	settingSeen := make(map[string]struct{})
	for _, item := range items {
		ws := strings.TrimSpace(item.WorkspaceID)
		if ws != "" {
			if _, ok := workspaceSeen[ws]; !ok {
				workspaceSeen[ws] = struct{}{}
				workspaceIDs = append(workspaceIDs, ws)
			}
		}
		stg := strings.TrimSpace(item.SettingID)
		if stg != "" {
			if _, ok := settingSeen[stg]; !ok {
				settingSeen[stg] = struct{}{}
				settingIDs = append(settingIDs, stg)
			}
		}
	}

	wsNameMap := make(map[string]string)
	if len(workspaceIDs) > 0 {
		type workspaceRow struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		var rows []workspaceRow
		if err := s.db.WithContext(ctx).Table("workspace").Select("id, name").Where("id IN ?", workspaceIDs).Find(&rows).Error; err == nil {
			for _, row := range rows {
				wsNameMap[strings.TrimSpace(row.ID)] = strings.TrimSpace(row.Name)
			}
		}
	}

	stgNameMap := make(map[string]string)
	if len(settingIDs) > 0 {
		type settingRow struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:storage_setting_name"`
		}
		var rows []settingRow
		if err := s.db.WithContext(ctx).Table("storage_settings").Select("id, storage_setting_name").Where("id IN ?", settingIDs).Find(&rows).Error; err == nil {
			for _, row := range rows {
				stgNameMap[strings.TrimSpace(row.ID)] = strings.TrimSpace(row.Name)
			}
		}
	}

	for i := range items {
		items[i].WorkspaceName = wsNameMap[strings.TrimSpace(items[i].WorkspaceID)]
		items[i].SettingName = stgNameMap[strings.TrimSpace(items[i].SettingID)]
	}
}

func (s *ShareService) loadShareFileIDs(ctx context.Context, shareID string) ([]string, error) {
	ids := make([]string, 0)
	if err := s.db.WithContext(ctx).Model(&sharedb.ShareItem{}).Where("share_id = ?", shareID).Pluck("file_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("query share items failed: %w", err)
	}
	return ids, nil
}

func (s *ShareService) isFileInShare(ctx context.Context, shareID, fileID string) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&sharedb.ShareItem{}).Where("share_id = ? AND file_id = ?", shareID, fileID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("query share relation failed: %w", err)
	}
	return count, nil
}

func (s *ShareService) createAccessRecord(ctx context.Context, shareID string, r *http.Request) error {
	if r == nil {
		return nil
	}
	ip := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
	}
	ua := strings.TrimSpace(r.UserAgent())
	address := "unknown"
	item := &sharedb.ShareAccessRecord{
		ShareID:       shareID,
		AccessIP:      ip,
		AccessAddress: address,
		Browser:       ua,
		Os:            ua,
		AccessTime:    time.Now(),
	}
	return s.db.WithContext(ctx).Create(item).Error
}
