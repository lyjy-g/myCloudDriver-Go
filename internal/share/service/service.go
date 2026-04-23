package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
)

// ShareService 是分享模块唯一实现。
type ShareService struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *ShareService {
	return &ShareService{db: db}
}

func (s *ShareService) Ping(_ context.Context) (string, error) {
	return "share service ready", nil
}

func currentUserID(ctx context.Context) (string, error) {
	p, ok := security.PrincipalFromContext(ctx)
	if !ok || strings.TrimSpace(p.UserID) == "" {
		return "", code.New(code.NoPermission, "login required")
	}
	return p.UserID, nil
}

func toShareVO(row FileShare, fileIDs []string) ShareVO {
	allowDownload := strings.Contains(strings.ToLower(row.Scope), "download")
	shareCode := ""
	if row.ShareCode != nil {
		shareCode = *row.ShareCode
	}
	return ShareVO{
		ShareID:       row.ID,
		ShareName:     row.ShareName,
		ShareCode:     shareCode,
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

func statusFromShare(share FileShare) int {
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
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.FileIDs) == 0 {
		return nil, code.New(code.BadRequest, "fileIds is required")
	}

	fileRows := make([]FileInfo, 0)
	if err = s.db.WithContext(ctx).Where("id IN ?", req.FileIDs).Find(&fileRows).Error; err != nil {
		return nil, fmt.Errorf("query files failed: %w", err)
	}
	if len(fileRows) == 0 {
		return nil, code.New(code.BadRequest, "shared files not found")
	}

	shareName := strings.TrimSpace(req.ShareName)
	if shareName == "" {
		shareName = fileRows[0].Name
		if len(fileRows) > 1 {
			shareName = fmt.Sprintf("%s 等%d个文件", fileRows[0].Name, len(fileRows))
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
	share := &FileShare{
		ID:         id,
		UserID:     userID,
		ShareName:  shareName,
		ShareCode:  &shareCode,
		ExpireTime: expireAt,
		Scope:      normalizeScope(allowDownload),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if errTx := tx.Create(share).Error; errTx != nil {
			return errTx
		}
		items := make([]FileShareItem, 0, len(req.FileIDs))
		for _, fid := range req.FileIDs {
			items = append(items, FileShareItem{ShareID: id, FileID: fid, CreatedAt: now})
		}
		if errTx := tx.Create(&items).Error; errTx != nil {
			return errTx
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create share failed: %w", err)
	}

	vo := toShareVO(*share, req.FileIDs)
	return &vo, nil
}

func (s *ShareService) ListMyShares(ctx context.Context) ([]ShareVO, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]FileShare, 0)
	if err = s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query shares failed: %w", err)
	}

	result := make([]ShareVO, 0, len(rows))
	for _, row := range rows {
		ids, _ := s.loadShareFileIDs(ctx, row.ID)
		result = append(result, toShareVO(row, ids))
	}
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
		if errTx := tx.Model(&FileShare{}).Where("id = ?", shareID).Updates(updates).Error; errTx != nil {
			return errTx
		}
		if len(req.FileIDs) > 0 {
			if errTx := tx.Where("share_id = ?", shareID).Delete(&FileShareItem{}).Error; errTx != nil {
				return errTx
			}
			items := make([]FileShareItem, 0, len(req.FileIDs))
			now := time.Now()
			for _, fid := range req.FileIDs {
				items = append(items, FileShareItem{ShareID: shareID, FileID: fid, CreatedAt: now})
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
	expect := ""
	if row.ShareCode != nil {
		expect = strings.TrimSpace(*row.ShareCode)
	}
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
	_ = s.db.WithContext(ctx).Model(&FileShare{}).Where("id = ?", shareID).Update("view_count", gorm.Expr("view_count + 1")).Error
	_ = s.createAccessRecord(ctx, shareID, r)

	ids := make([]string, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.FileID)
	}
	vo := toShareVO(*row, ids)
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
	rows := make([]FileInfo, 0)
	q := s.db.WithContext(ctx).Where("id IN ?", fileIDs)
	if strings.TrimSpace(parentID) != "" {
		q = q.Where("parent_id = ?", parentID)
	}
	if err = q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query shared files failed: %w", err)
	}
	items := make([]ShareFileVO, 0, len(rows))
	for _, row := range rows {
		item := ShareFileVO{FileID: row.ID, FileName: row.Name, FileSize: row.Size, Directory: row.IsDir}
		if !row.IsDir {
			item.DownloadURL = "/apis/shares/public/" + shareID + "/download/" + row.ID
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *ShareService) DownloadShareFile(ctx context.Context, shareID, fileID, shareCode string, r *http.Request) ([]byte, string, error) {
	ok, err := s.VerifyShareCode(ctx, shareID, shareCode)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", code.New(code.BadRequest, "share code verify failed")
	}
	count, err := s.isFileInShare(ctx, shareID, fileID)
	if err != nil {
		return nil, "", err
	}
	if count == 0 {
		return nil, "", code.New(code.NoPermission, "file not in share")
	}

	var file FileInfo
	if err = s.db.WithContext(ctx).Where("id = ?", fileID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", code.New(code.NotFound, "file not found")
		}
		return nil, "", fmt.Errorf("query file failed: %w", err)
	}
	_ = s.db.WithContext(ctx).Model(&FileShare{}).Where("id = ?", shareID).Update("download_count", gorm.Expr("download_count + 1")).Error
	_ = s.createAccessRecord(ctx, shareID, r)
	content := []byte("download placeholder for file=" + file.ID + " name=" + file.Name)
	return content, file.Name, nil
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
		if errTx := tx.Where("share_id IN ?", shareIDs).Delete(&FileShareItem{}).Error; errTx != nil {
			return errTx
		}
		if errTx := tx.Where("id IN ? AND user_id = ?", shareIDs, uid).Delete(&FileShare{}).Error; errTx != nil {
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
	if err = s.db.WithContext(ctx).Model(&FileShare{}).Where("user_id = ?", uid).Pluck("id", &ids).Error; err != nil {
		return fmt.Errorf("query user shares failed: %w", err)
	}
	return s.CancelShares(ctx, ids)
}

func (s *ShareService) getShare(ctx context.Context, shareID string) (*FileShare, error) {
	row := &FileShare{}
	if err := s.db.WithContext(ctx).Where("id = ?", shareID).First(row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, code.New(code.NotFound, "share not found")
		}
		return nil, fmt.Errorf("query share failed: %w", err)
	}
	return row, nil
}

func (s *ShareService) loadShareFileIDs(ctx context.Context, shareID string) ([]string, error) {
	ids := make([]string, 0)
	if err := s.db.WithContext(ctx).Model(&FileShareItem{}).Where("share_id = ?", shareID).Pluck("file_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("query share items failed: %w", err)
	}
	return ids, nil
}

func (s *ShareService) isFileInShare(ctx context.Context, shareID, fileID string) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&FileShareItem{}).Where("share_id = ? AND file_id = ?", shareID, fileID).Count(&count).Error; err != nil {
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
	item := &FileShareAccessRecord{
		ShareID:       shareID,
		AccessIP:      &ip,
		AccessAddress: &address,
		Browser:       &ua,
		OS:            &ua,
		AccessTime:    time.Now(),
	}
	return s.db.WithContext(ctx).Create(item).Error
}
