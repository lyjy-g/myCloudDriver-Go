package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
)

// LoginReq 登录请求。
type LoginReq struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	IsRemember bool   `json:"isRemember"`
}

// RegisterReq 注册请求。
type RegisterReq struct {
	Username        string  `json:"username"`
	Password        string  `json:"password"`
	ConfirmPassword string  `json:"confirmPassword"`
	Email           string  `json:"email"`
	Nickname        string  `json:"nickname"`
	Avatar          *string `json:"avatar"`
}

// UpdateUserReq 编辑用户请求。
type UpdateUserReq struct {
	Nickname string  `json:"nickname"`
	Avatar   *string `json:"avatar"`
}

// ChangePasswordReq 修改密码请求。
type ChangePasswordReq struct {
	OldPassword     string `json:"oldPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

// ForgetPasswordReq 忘记密码重置请求。
type ForgetPasswordReq struct {
	Mail            string `json:"mail"`
	Code            string `json:"code"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

// Login 执行登录。
func (s *UserService) Login(ctx context.Context, req LoginReq, r *http.Request) (*LoginResult, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" || strings.TrimSpace(req.Password) == "" {
		_ = s.writeLoginLog(ctx, nil, username, r, 1, "username/password required")
		return nil, code.New(code.BadRequest, "username/password required")
	}

	var user SysUser
	err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		_ = s.writeLoginLog(ctx, nil, username, r, 1, "account not found")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, code.New(code.BadRequest, "username or password invalid")
		}
		return nil, fmt.Errorf("query user failed: %w", err)
	}

	if user.Status != 0 {
		_ = s.writeLoginLog(ctx, &user.ID, user.Username, r, 1, "account disabled")
		return nil, code.New(code.NoPermission, "account disabled")
	}
	if hashPassword(req.Password) != user.Password {
		_ = s.writeLoginLog(ctx, &user.ID, user.Username, r, 1, "password invalid")
		return nil, code.New(code.BadRequest, "username or password invalid")
	}

	workspaceID := ""
	if user.DefaultWorkspaceID != nil {
		workspaceID = *user.DefaultWorkspaceID
	}
	if workspaceID == "" {
		workspaceID = "ws_" + user.ID + "_personal"
	}

	ttl := s.tokenTTL
	if req.IsRemember {
		ttl = s.rememberTTL
	}
	token, err := s.jwt.Issue(user.ID, workspaceID, user.Username, ttl)
	if err != nil {
		return nil, fmt.Errorf("issue token failed: %w", err)
	}

	now := time.Now()
	if err = s.db.WithContext(ctx).Model(&SysUser{}).Where("id = ?", user.ID).Updates(map[string]any{
		"last_login_at": now,
		"updated_at":    now,
	}).Error; err != nil {
		return nil, fmt.Errorf("update login time failed: %w", err)
	}
	_ = s.writeLoginLog(ctx, &user.ID, user.Username, r, 0, "success")

	return &LoginResult{
		Token:       token,
		UserID:      user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Email:       user.Email,
		Avatar:      user.Avatar,
		WorkspaceID: workspaceID,
	}, nil
}

// Logout 执行登出，支持 token 黑名单。
func (s *UserService) Logout(ctx context.Context) error {
	p, err := requirePrincipal(ctx)
	if err != nil {
		return nil
	}
	if strings.TrimSpace(p.Token) == "" {
		return nil
	}
	claims, parseErr := s.jwt.Parse(p.Token)
	if parseErr != nil {
		return nil
	}
	return security.BlacklistToken(ctx, s.rdb, p.Token, claims.ExpireAt)
}

// CurrentUser 获取当前用户详情。
func (s *UserService) CurrentUser(ctx context.Context) (*UserInfo, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}

	var user SysUser
	if err = s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, code.New(code.NotFound, "user not found")
		}
		return nil, fmt.Errorf("query user failed: %w", err)
	}

	defaultWorkspaceID := ""
	if user.DefaultWorkspaceID != nil {
		defaultWorkspaceID = *user.DefaultWorkspaceID
	}
	return &UserInfo{
		ID:                 user.ID,
		Username:           user.Username,
		Email:              user.Email,
		Nickname:           user.Nickname,
		Avatar:             user.Avatar,
		DefaultWorkspaceID: defaultWorkspaceID,
		Status:             user.Status,
	}, nil
}

// Register 注册用户并初始化个人工作空间与传输设置。
func (s *UserService) Register(ctx context.Context, req RegisterReq) error {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return code.New(code.BadRequest, "username is required")
	}
	if err := assertPasswordPair(req.Password, req.ConfirmPassword); err != nil {
		return err
	}
	if err := validateEmail(req.Email); err != nil {
		return err
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = username
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&SysUser{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return fmt.Errorf("check username failed: %w", err)
	}
	if count > 0 {
		return code.New(code.BadRequest, "username already exists")
	}

	userID := randomID("usr")
	workspaceID := "ws_" + userID + "_personal"
	now := time.Now()
	user := &SysUser{
		ID:                 userID,
		Username:           username,
		Password:           hashPassword(req.Password),
		Email:              strings.TrimSpace(req.Email),
		Nickname:           nickname,
		Avatar:             req.Avatar,
		DefaultWorkspaceID: &workspaceID,
		Status:             0,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("create user failed: %w", err)
		}
		ws := &Workspace{
			ID:            workspaceID,
			Name:          username + " 个人空间",
			WorkspaceType: "personal",
			OwnerUserID:   userID,
			Status:        1,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(ws).Error; err != nil {
			return fmt.Errorf("create personal workspace failed: %w", err)
		}
		member := &WorkspaceMember{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        "owner",
			Status:      1,
			JoinedAt:    now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.Create(member).Error; err != nil {
			return fmt.Errorf("init workspace owner failed: %w", err)
		}
		if err := (&UserService{db: tx, rdb: s.rdb, jwt: s.jwt}).mustInitTransferSetting(ctx, userID); err != nil {
			return err
		}
		return nil
	})
}

// UpdateUserInfo 更新用户资料。
func (s *UserService) UpdateUserInfo(ctx context.Context, req UpdateUserReq) error {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return err
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		return code.New(code.BadRequest, "nickname is required")
	}

	updates := map[string]any{"nickname": nickname, "updated_at": time.Now()}
	if req.Avatar != nil {
		updates["avatar"] = req.Avatar
	}
	result := s.db.WithContext(ctx).Model(&SysUser{}).Where("id = ?", userID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update user failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return code.New(code.NotFound, "user not found")
	}
	return nil
}

// ChangePassword 登录态改密。
func (s *UserService) ChangePassword(ctx context.Context, req ChangePasswordReq) error {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return err
	}
	if err := assertPasswordPair(req.NewPassword, req.ConfirmPassword); err != nil {
		return err
	}

	var user SysUser
	if err = s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code.New(code.NotFound, "user not found")
		}
		return fmt.Errorf("query user failed: %w", err)
	}
	if hashPassword(req.OldPassword) != user.Password {
		return code.New(code.BadRequest, "old password incorrect")
	}

	return s.db.WithContext(ctx).Model(&SysUser{}).Where("id = ?", userID).Updates(map[string]any{
		"password":   hashPassword(req.NewPassword),
		"updated_at": time.Now(),
	}).Error
}

// SendForgetPasswordCode 发送忘记密码验证码（演示实现：返回成功并写 Redis）。
func (s *UserService) SendForgetPasswordCode(ctx context.Context, mail string) error {
	if err := validateEmail(mail); err != nil {
		return err
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&SysUser{}).Where("email = ?", strings.TrimSpace(mail)).Count(&count).Error; err != nil {
		return fmt.Errorf("query user by email failed: %w", err)
	}
	if count == 0 {
		return code.New(code.NotFound, "email not found")
	}

	codeValue := randomCode(6)
	if s.rdb != nil {
		if err := s.rdb.Set(ctx, forgetCodeCacheKey(mail), codeValue, s.forgotCodeTTL).Err(); err != nil {
			return fmt.Errorf("save code to redis failed: %w", err)
		}
	}
	return nil
}

// ResetForgetPassword 验证码重置密码。
func (s *UserService) ResetForgetPassword(ctx context.Context, req ForgetPasswordReq) error {
	if err := validateEmail(req.Mail); err != nil {
		return err
	}
	if err := assertPasswordPair(req.NewPassword, req.ConfirmPassword); err != nil {
		return err
	}
	if strings.TrimSpace(req.Code) == "" {
		return code.New(code.BadRequest, "code is required")
	}

	if s.rdb == nil {
		return code.New(code.InternalError, "redis unavailable")
	}
	cachedCode, err := s.rdb.Get(ctx, forgetCodeCacheKey(req.Mail)).Result()
	if err != nil {
		return code.New(code.BadRequest, "code invalid or expired")
	}
	if strings.TrimSpace(cachedCode) != strings.TrimSpace(req.Code) {
		return code.New(code.BadRequest, "code invalid or expired")
	}

	result := s.db.WithContext(ctx).Model(&SysUser{}).Where("email = ?", strings.TrimSpace(req.Mail)).Updates(map[string]any{
		"password":   hashPassword(req.NewPassword),
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return fmt.Errorf("reset password failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return code.New(code.NotFound, "email not found")
	}
	_ = s.rdb.Del(ctx, forgetCodeCacheKey(req.Mail)).Err()
	return nil
}

// GetTransferSetting 查询用户传输设置。
func (s *UserService) GetTransferSetting(ctx context.Context) (*SysUserTransferSetting, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.mustInitTransferSetting(ctx, userID); err != nil {
		return nil, err
	}
	var item SysUserTransferSetting
	if err = s.db.WithContext(ctx).Where("user_id = ?", userID).First(&item).Error; err != nil {
		return nil, fmt.Errorf("query transfer setting failed: %w", err)
	}
	return &item, nil
}

// UpdateTransferSetting 更新用户传输设置。
func (s *UserService) UpdateTransferSetting(ctx context.Context, req TransferSettingInput) (*SysUserTransferSetting, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.ConcurrentUploadQuantity <= 0 || req.ConcurrentDownloadQuantity <= 0 || req.ChunkSize <= 0 {
		return nil, code.New(code.BadRequest, "invalid transfer setting")
	}
	if err = s.mustInitTransferSetting(ctx, userID); err != nil {
		return nil, err
	}

	updates := map[string]any{
		"download_location":            req.DownloadLocation,
		"is_default_download_location": req.IsDefaultDownloadLocation,
		"download_speed_limit":         req.DownloadSpeedLimit,
		"concurrent_upload_quantity":   req.ConcurrentUploadQuantity,
		"concurrent_download_quantity": req.ConcurrentDownloadQuantity,
		"chunk_size":                   req.ChunkSize,
		"updated_at":                   time.Now(),
	}
	if err = s.db.WithContext(ctx).Model(&SysUserTransferSetting{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update transfer setting failed: %w", err)
	}
	return s.GetTransferSetting(ctx)
}

// ListWorkspaces 查询当前用户可访问工作空间。
func (s *UserService) ListWorkspaces(ctx context.Context) ([]WorkspaceItem, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	var user SysUser
	if err = s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, fmt.Errorf("query user failed: %w", err)
	}

	type row struct {
		ID            string
		Name          string
		WorkspaceType string
		Role          string
	}
	rows := make([]row, 0)
	if err = s.db.WithContext(ctx).Table("workspace w").
		Select("w.id, w.name, w.workspace_type, wm.role").
		Joins("join workspace_member wm on wm.workspace_id = w.id and wm.status = 1").
		Where("wm.user_id = ? and w.status = 1", userID).
		Order("w.created_at asc").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query workspaces failed: %w", err)
	}

	defaultWS := ""
	if user.DefaultWorkspaceID != nil {
		defaultWS = *user.DefaultWorkspaceID
	}
	items := make([]WorkspaceItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, WorkspaceItem{
			ID:            r.ID,
			Name:          r.Name,
			WorkspaceType: r.WorkspaceType,
			Role:          r.Role,
			IsDefault:     r.ID == defaultWS,
		})
	}
	return items, nil
}

// CreateOrgWorkspace 创建组织空间并把当前用户设置为 owner。
func (s *UserService) CreateOrgWorkspace(ctx context.Context, name string) (*WorkspaceItem, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, code.New(code.BadRequest, "workspace name is required")
	}
	workspaceID := randomID("ws")
	now := time.Now()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ws := &Workspace{
			ID:            workspaceID,
			Name:          name,
			WorkspaceType: "org",
			OwnerUserID:   userID,
			Status:        1,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(ws).Error; err != nil {
			return err
		}
		member := &WorkspaceMember{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        "owner",
			Status:      1,
			JoinedAt:    now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		return tx.Create(member).Error
	})
	if err != nil {
		return nil, fmt.Errorf("create org workspace failed: %w", err)
	}
	return &WorkspaceItem{ID: workspaceID, Name: name, WorkspaceType: "org", Role: "owner", IsDefault: false}, nil
}

// SetDefaultWorkspace 设置默认工作空间。
func (s *UserService) SetDefaultWorkspace(ctx context.Context, workspaceID string) error {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return code.New(code.BadRequest, "workspaceId is required")
	}

	var count int64
	if err = s.db.WithContext(ctx).Model(&WorkspaceMember{}).Where("workspace_id = ? and user_id = ? and status = 1", workspaceID, userID).Count(&count).Error; err != nil {
		return fmt.Errorf("query workspace member failed: %w", err)
	}
	if count == 0 {
		return code.New(code.NoPermission, "workspace not accessible")
	}

	if err = s.db.WithContext(ctx).Model(&SysUser{}).Where("id = ?", userID).Updates(map[string]any{
		"default_workspace_id": workspaceID,
		"updated_at":           time.Now(),
	}).Error; err != nil {
		return fmt.Errorf("set default workspace failed: %w", err)
	}
	return nil
}

// ListWorkspaceMembers 查询空间成员。
func (s *UserService) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]WorkspaceMemberItem, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.assertWorkspaceMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}

	rows := make([]WorkspaceMemberItem, 0)
	if err = s.db.WithContext(ctx).Table("workspace_member").
		Select("user_id, role, joined_at").
		Where("workspace_id = ? and status = 1", workspaceID).
		Order("joined_at asc").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query workspace members failed: %w", err)
	}
	return rows, nil
}

// AddWorkspaceMember 添加空间成员。
func (s *UserService) AddWorkspaceMember(ctx context.Context, workspaceID, targetUserID, role string) error {
	operatorID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return err
	}
	if err = s.assertWorkspaceManagePermission(ctx, workspaceID, operatorID); err != nil {
		return err
	}
	targetUserID = strings.TrimSpace(targetUserID)
	role = strings.TrimSpace(strings.ToLower(role))
	if targetUserID == "" {
		return code.New(code.BadRequest, "userId is required")
	}
	if role != "owner" && role != "admin" && role != "member" {
		return code.New(code.BadRequest, "role must be owner/admin/member")
	}

	now := time.Now()
	member := &WorkspaceMember{WorkspaceID: workspaceID, UserID: targetUserID, Role: role, Status: 1, JoinedAt: now, CreatedAt: now, UpdatedAt: now}
	if err = s.db.WithContext(ctx).Where("workspace_id = ? and user_id = ?", workspaceID, targetUserID).Assign(member).FirstOrCreate(member).Error; err != nil {
		return fmt.Errorf("add workspace member failed: %w", err)
	}
	return nil
}

// RemoveWorkspaceMember 移除空间成员。
func (s *UserService) RemoveWorkspaceMember(ctx context.Context, workspaceID, targetUserID string) error {
	operatorID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return err
	}
	if err = s.assertWorkspaceManagePermission(ctx, workspaceID, operatorID); err != nil {
		return err
	}

	result := s.db.WithContext(ctx).Model(&WorkspaceMember{}).
		Where("workspace_id = ? and user_id = ?", workspaceID, targetUserID).
		Updates(map[string]any{"status": 0, "updated_at": time.Now()})
	if result.Error != nil {
		return fmt.Errorf("remove workspace member failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return code.New(code.NotFound, "member not found")
	}
	return nil
}

func (s *UserService) assertWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&WorkspaceMember{}).Where("workspace_id = ? and user_id = ? and status = 1", workspaceID, userID).Count(&count).Error; err != nil {
		return fmt.Errorf("query workspace member failed: %w", err)
	}
	if count == 0 {
		return code.New(code.NoPermission, "workspace not accessible")
	}
	return nil
}

func (s *UserService) assertWorkspaceManagePermission(ctx context.Context, workspaceID, userID string) error {
	type row struct{ Role string }
	var r row
	err := s.db.WithContext(ctx).Model(&WorkspaceMember{}).
		Select("role").Where("workspace_id = ? and user_id = ? and status = 1", workspaceID, userID).
		Take(&r).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code.New(code.NoPermission, "workspace not accessible")
		}
		return fmt.Errorf("query workspace role failed: %w", err)
	}
	if r.Role != "owner" && r.Role != "admin" {
		return code.New(code.NoPermission, "workspace manage permission required")
	}
	return nil
}

func (s *UserService) writeLoginLog(ctx context.Context, userID *string, username string, r *http.Request, status int, msg string) error {
	if strings.TrimSpace(username) == "" {
		username = "unknown"
	}
	ip := ""
	if r != nil {
		ip = r.Header.Get("X-Forwarded-For")
		if strings.TrimSpace(ip) == "" {
			ip, _, _ = net.SplitHostPort(r.RemoteAddr)
			if ip == "" {
				ip = r.RemoteAddr
			}
		}
	}
	item := &LoginLog{
		UserID:    userID,
		Username:  username,
		LoginIP:   ip,
		OS:        "unknown",
		Status:    status,
		Msg:       msg,
		LoginTime: time.Now(),
	}
	return s.db.WithContext(ctx).Create(item).Error
}
