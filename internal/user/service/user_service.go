package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	userapi "myclouddrive-go/internal/user/api/gen"
	usermodel "myclouddrive-go/internal/user/model"
	"myclouddrive-go/internal/user/model/dbmodel"
)

const (
	// defaultTokenTTL 普通登录 token 过期时间。
	defaultTokenTTL = 24 * time.Hour
	// rememberTokenTTL 勾选“记住我”后的 token 过期时间。
	rememberTokenTTL = 7 * 24 * time.Hour
	// defaultForgotCodeTTL 忘记密码验证码默认有效期。
	defaultForgotCodeTTL = 10 * time.Minute
	// defaultChunkSize 传输默认分片大小（5MB）。
	defaultChunkSize = int64(5 * 1024 * 1024)
)

// emailPattern 用于基础邮箱格式校验（仅做格式约束，不做域名可达性校验）。
var emailPattern = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)

// UserService 是 user 模块的唯一实现。
type UserService struct {
	db            *gorm.DB
	rdb           redis.Cmdable
	jwt           *security.JWTService
	tokenTTL      time.Duration
	rememberTTL   time.Duration
	forgotCodeTTL time.Duration
}

// NewUserService 创建用户服务。
func NewUserService(db *gorm.DB, rdb redis.Cmdable, jwt *security.JWTService) *UserService {
	return &UserService{
		db:            db,
		rdb:           rdb,
		jwt:           jwt,
		tokenTTL:      defaultTokenTTL,
		rememberTTL:   rememberTokenTTL,
		forgotCodeTTL: defaultForgotCodeTTL,
	}
}

// hashPassword 使用 SHA-256 对明文密码做摘要。
// 注意：这里为兼容历史逻辑，未引入盐值和慢哈希算法。
func hashPassword(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// randomCode 生成 n 位数字验证码。
// 当 n 非法时回退到 6 位。
func randomCode(n int) string {
	if n <= 0 {
		n = 6
	}
	randSrc := rand.New(rand.NewSource(time.Now().UnixNano()))
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte('0' + randSrc.Intn(10))
	}
	return string(buf)
}

// randomID 生成带前缀的简单唯一 ID（基于纳秒时间戳）。
func randomID(prefix string) string {
	if strings.TrimSpace(prefix) == "" {
		prefix = "id"
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// requirePrincipal 从上下文中提取登录主体并做登录态校验。
func requirePrincipal(ctx context.Context) (security.Principal, error) {
	p, ok := security.PrincipalFromContext(ctx)
	if !ok || strings.TrimSpace(p.UserID) == "" {
		return security.Principal{}, code.New(code.NoPermission, "login required")
	}
	return p, nil
}

// resolveCurrentUserID 解析当前登录用户 ID。
func resolveCurrentUserID(ctx context.Context) (string, error) {
	p, err := requirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	return p.UserID, nil
}

// resolveCurrentWorkspaceID 解析当前上下文中的工作空间 ID。
// 当前文件中未直接使用，保留供后续扩展。
func resolveCurrentWorkspaceID(ctx context.Context) string {
	if p, ok := security.PrincipalFromContext(ctx); ok && strings.TrimSpace(p.WorkspaceID) != "" {
		return p.WorkspaceID
	}
	return ""
}

// validateEmail 校验邮箱格式。
func validateEmail(mail string) error {
	mail = strings.TrimSpace(mail)
	if mail == "" {
		return code.New(code.BadRequest, "email is required")
	}
	if !emailPattern.MatchString(mail) {
		return code.New(code.BadRequest, "invalid email format")
	}
	return nil
}

// assertPasswordPair 校验密码与确认密码。
func assertPasswordPair(password, confirm string) error {
	if strings.TrimSpace(password) == "" {
		return code.New(code.BadRequest, "password is required")
	}
	if password != confirm {
		return code.New(code.BadRequest, "password and confirmPassword not match")
	}
	return nil
}

// strPtrOrNil 将非空字符串转换为指针，空字符串返回 nil。
func strPtrOrNil(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := s
	return &v
}

// timePtrOrNil 将非零时间转换为指针，零值时间返回 nil。
func timePtrOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t
	return &v
}

// ptrString 解引用字符串指针，nil 返回空字符串。
func ptrString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// boolToInt32 将布尔值转换为 0/1，便于与历史 API 字段兼容。
func boolToInt32(v bool) int32 {
	if v {
		return 1
	}
	return 0
}

// toSysUserVO 将 dbmodel.User 映射为 OpenAPI 生成的返回对象。
func toSysUserVO(user dbmodel.User) *userapi.SysUserVO {
	return &userapi.SysUserVO{
		Id:          strPtrOrNil(user.ID),
		Username:    strPtrOrNil(user.Username),
		Email:       strPtrOrNil(user.Email),
		Nickname:    strPtrOrNil(user.Nickname),
		Avatar:      strPtrOrNil(user.Avatar),
		Status:      &user.Status,
		CreatedAt:   timePtrOrNil(user.CreatedAt),
		UpdatedAt:   timePtrOrNil(user.UpdatedAt),
		LastLoginAt: timePtrOrNil(user.LastLoginAt),
	}
}

// toTransferSettingVO 将 dbmodel.UserTransferSetting 映射为 OpenAPI 返回对象。
func toTransferSettingVO(item dbmodel.UserTransferSetting) *userapi.SysUserTransferSetting {
	upload := item.ConcurrentUploadQuantity
	download := item.ConcurrentDownloadQuantity
	speed := item.DownloadSpeedLimit
	defaultDownloadLocation := boolToInt32(item.IsDefaultDownloadLocation)
	return &userapi.SysUserTransferSetting{
		Id:                         &item.ID,
		UserId:                     strPtrOrNil(item.UserID),
		DownloadLocation:           strPtrOrNil(item.DownloadLocation),
		IsDefaultDownloadLocation:  &defaultDownloadLocation,
		DownloadSpeedLimit:         &speed,
		ConcurrentUploadQuantity:   &upload,
		ConcurrentDownloadQuantity: &download,
		ChunkSize:                  &item.ChunkSize,
		CreatedAt:                  timePtrOrNil(item.CreatedAt),
		UpdatedAt:                  timePtrOrNil(item.UpdatedAt),
	}
}

// mustInitTransferSetting 确保用户存在传输配置。
// 若不存在则按系统默认值初始化一条记录。
func (s *UserService) mustInitTransferSetting(ctx context.Context, userID string) error {
	var item dbmodel.UserTransferSetting
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&item).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query transfer setting failed: %w", err)
	}

	now := time.Now()
	create := &dbmodel.UserTransferSetting{
		UserID:                     userID,
		DownloadLocation:           "",
		IsDefaultDownloadLocation:  false,
		DownloadSpeedLimit:         5,
		ConcurrentUploadQuantity:   1,
		ConcurrentDownloadQuantity: 1,
		ChunkSize:                  defaultChunkSize,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}
	if err = s.db.WithContext(ctx).Create(create).Error; err != nil {
		return fmt.Errorf("init transfer setting failed: %w", err)
	}
	return nil
}

// forgetCodeCacheKey 生成忘记密码验证码的 Redis Key。
func forgetCodeCacheKey(mail string) string {
	return "user:forget:code:" + strings.ToLower(strings.TrimSpace(mail))
}

// Login 执行登录。
func (s *UserService) Login(ctx context.Context, req userapi.LoginCmd, r *http.Request) (*usermodel.LoginResult, error) {
	username := strings.TrimSpace(req.Username)

	if username == "" || strings.TrimSpace(req.Password) == "" {
		return nil, code.New(code.BadRequest, "username/password required")
	}

	var user dbmodel.User

	err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, code.New(code.BadRequest, "username or password invalid")
		}
		return nil, fmt.Errorf("query user failed: %w", err)
	}

	if user.Status != 0 {
		return nil, code.New(code.NoPermission, "account disabled")
	}
	if hashPassword(req.Password) != user.Password {
		return nil, code.New(code.BadRequest, "username or password invalid")
	}

	workspaceID := ""
	// 默认工作空间为空时，回退到个人空间约定 ID，保证 token 内 workspace 可用。
	workspaceID = strings.TrimSpace(user.DefaultWorkspaceID)
	if workspaceID == "" {
		workspaceID = "ws_" + user.ID + "_personal"
	}

	ttl := s.tokenTTL
	// 记住我场景下发更长过期时间的 token。
	if req.IsRemember != nil && *req.IsRemember {
		ttl = s.rememberTTL
	}
	token, err := s.jwt.GetToken(user.ID, workspaceID, user.Username, ttl)
	if err != nil {
		return nil, fmt.Errorf("issue token failed: %w", err)
	}

	now := time.Now()
	// 登录成功后刷新用户最后登录时间，便于审计与风控。
	if err = s.db.WithContext(ctx).Model(&dbmodel.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"last_login_at": now,
		"updated_at":    now,
	}).Error; err != nil {
		return nil, fmt.Errorf("update login time failed: %w", err)
	}

	return &usermodel.LoginResult{
		Token:       token,
		UserID:      user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Email:       user.Email,
		Avatar:      strPtrOrNil(user.Avatar),
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
	claims, parseErr := s.jwt.ParseToken(p.Token)
	if parseErr != nil {
		return nil
	}
	return security.BlacklistToken(ctx, s.rdb, p.Token, claims.ExpireAt)
}

// CurrentUser 获取当前用户详情。
func (s *UserService) CurrentUser(ctx context.Context) (*userapi.SysUserVO, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}

	var user dbmodel.User
	if err = s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, code.New(code.NotFound, "user not found")
		}
		return nil, fmt.Errorf("query user failed: %w", err)
	}

	return toSysUserVO(user), nil
}

// Register 注册用户并初始化个人工作空间与传输设置。
func (s *UserService) Register(ctx context.Context, req userapi.UserRegisterCmd) error {
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
	if err := s.db.WithContext(ctx).Model(&dbmodel.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return fmt.Errorf("check username failed: %w", err)
	}
	if count > 0 {
		return code.New(code.BadRequest, "username already exists")
	}

	userID := randomID("usr")
	workspaceID := "ws_" + userID + "_personal"
	now := time.Now()
	user := &dbmodel.User{
		ID:                 userID,
		Username:           username,
		Password:           hashPassword(req.Password),
		Email:              strings.TrimSpace(req.Email),
		Nickname:           nickname,
		Avatar:             strings.TrimSpace(ptrString(req.Avatar)),
		DefaultWorkspaceID: workspaceID,
		Status:             int32(0),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 注册流程采用事务，保证用户、个人空间、空间成员、传输配置一致性。
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("create user failed: %w", err)
		}
		ws := &dbmodel.Workspace{
			ID:            workspaceID,
			Name:          username + " 个人空间",
			WorkspaceType: "personal",
			OwnerUserID:   userID,
			Status:        true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(ws).Error; err != nil {
			return fmt.Errorf("create personal workspace failed: %w", err)
		}
		member := &dbmodel.WorkspaceMember{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        "owner",
			Status:      true,
			JoinedAt:    now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.Create(member).Error; err != nil {
			return fmt.Errorf("init workspace owner failed: %w", err)
		}
		// 复用服务内部初始化逻辑，避免重复实现默认配置。
		if err := (&UserService{db: tx, rdb: s.rdb, jwt: s.jwt}).mustInitTransferSetting(ctx, userID); err != nil {
			return err
		}
		return nil
	})
}

// UpdateUserInfo 更新用户资料。
func (s *UserService) UpdateUserInfo(ctx context.Context, req userapi.UserEditInfoCmd) error {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return err
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		return code.New(code.BadRequest, "nickname is required")
	}

	updates := map[string]any{"nickname": nickname, "updated_at": time.Now()}
	result := s.db.WithContext(ctx).Model(&dbmodel.User{}).Where("id = ?", userID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update user failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return code.New(code.NotFound, "user not found")
	}
	return nil
}

// ChangePassword 登录态改密。
func (s *UserService) ChangePassword(ctx context.Context, req userapi.PasswordEditCmd) error {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return err
	}
	if err := assertPasswordPair(req.NewPassword, req.ConfirmPassword); err != nil {
		return err
	}

	var user dbmodel.User
	if err = s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code.New(code.NotFound, "user not found")
		}
		return fmt.Errorf("query user failed: %w", err)
	}
	if hashPassword(req.OldPassword) != user.Password {
		return code.New(code.BadRequest, "old password incorrect")
	}

	return s.db.WithContext(ctx).Model(&dbmodel.User{}).Where("id = ?", userID).Updates(map[string]any{
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
	if err := s.db.WithContext(ctx).Model(&dbmodel.User{}).Where("email = ?", strings.TrimSpace(mail)).Count(&count).Error; err != nil {
		return fmt.Errorf("query user by email failed: %w", err)
	}
	if count == 0 {
		return code.New(code.NotFound, "email not found")
	}

	codeValue := randomCode(6)
	// 开发阶段仅写入 Redis，后续可在此接入短信/邮件网关。
	if s.rdb != nil {
		if err := s.rdb.Set(ctx, forgetCodeCacheKey(mail), codeValue, s.forgotCodeTTL).Err(); err != nil {
			return fmt.Errorf("save code to redis failed: %w", err)
		}
	}
	return nil
}

// ResetForgetPassword 验证码重置密码。
func (s *UserService) ResetForgetPassword(ctx context.Context, req userapi.PasswordForgetEditCmd) error {
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

	result := s.db.WithContext(ctx).Model(&dbmodel.User{}).Where("email = ?", strings.TrimSpace(req.Mail)).Updates(map[string]any{
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
func (s *UserService) GetTransferSetting(ctx context.Context) (*userapi.SysUserTransferSetting, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.mustInitTransferSetting(ctx, userID); err != nil {
		return nil, err
	}
	var item dbmodel.UserTransferSetting
	if err = s.db.WithContext(ctx).Where("user_id = ?", userID).First(&item).Error; err != nil {
		return nil, fmt.Errorf("query transfer setting failed: %w", err)
	}
	return toTransferSettingVO(item), nil
}

// UpdateTransferSetting 更新用户传输设置。
func (s *UserService) UpdateTransferSetting(ctx context.Context, req userapi.UserTransferSettingEditCmd) (*userapi.SysUserTransferSetting, error) {
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
		"is_default_download_location": req.IsDefaultDownloadLocation == 1,
		"download_speed_limit":         req.DownloadSpeedLimit,
		"concurrent_upload_quantity":   req.ConcurrentUploadQuantity,
		"concurrent_download_quantity": req.ConcurrentDownloadQuantity,
		"chunk_size":                   req.ChunkSize,
		"updated_at":                   time.Now(),
	}
	if err = s.db.WithContext(ctx).Model(&dbmodel.UserTransferSetting{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update transfer setting failed: %w", err)
	}
	return s.GetTransferSetting(ctx)
}

// ListWorkspaces 查询当前用户可访问工作空间。
func (s *UserService) ListWorkspaces(ctx context.Context) ([]usermodel.WorkspaceItem, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	var user dbmodel.User
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
	// 通过成员关系表过滤“当前用户可见空间”，并携带用户在空间内角色。
	if err = s.db.WithContext(ctx).Table("workspace w").
		Select("w.id, w.name, w.workspace_type, wm.role").
		Joins("join workspace_member wm on wm.workspace_id = w.id and wm.status = 1").
		Where("wm.user_id = ? and w.status = 1", userID).
		Order("w.created_at asc").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query workspaces failed: %w", err)
	}

	defaultWS := user.DefaultWorkspaceID
	items := make([]usermodel.WorkspaceItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, usermodel.WorkspaceItem{
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
func (s *UserService) CreateOrgWorkspace(ctx context.Context, name string) (*usermodel.WorkspaceItem, error) {
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
		ws := &dbmodel.Workspace{
			ID:            workspaceID,
			Name:          name,
			WorkspaceType: "org",
			OwnerUserID:   userID,
			Status:        true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(ws).Error; err != nil {
			return err
		}
		member := &dbmodel.WorkspaceMember{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        "owner",
			Status:      true,
			JoinedAt:    now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		return tx.Create(member).Error
	})
	if err != nil {
		return nil, fmt.Errorf("create org workspace failed: %w", err)
	}
	return &usermodel.WorkspaceItem{ID: workspaceID, Name: name, WorkspaceType: "org", Role: "owner", IsDefault: false}, nil
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
	if err = s.db.WithContext(ctx).Model(&dbmodel.WorkspaceMember{}).Where("workspace_id = ? and user_id = ? and status = 1", workspaceID, userID).Count(&count).Error; err != nil {
		return fmt.Errorf("query workspace member failed: %w", err)
	}
	if count == 0 {
		return code.New(code.NoPermission, "workspace not accessible")
	}

	if err = s.db.WithContext(ctx).Model(&dbmodel.User{}).Where("id = ?", userID).Updates(map[string]any{
		"default_workspace_id": workspaceID,
		"updated_at":           time.Now(),
	}).Error; err != nil {
		return fmt.Errorf("set default workspace failed: %w", err)
	}
	return nil
}

// ListWorkspaceMembers 查询空间成员。
func (s *UserService) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]usermodel.WorkspaceMemberItem, error) {
	userID, err := resolveCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.assertWorkspaceMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}

	rows := make([]usermodel.WorkspaceMemberItem, 0)
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
	// 统一 role 入参格式，避免大小写导致的分支遗漏。
	targetUserID = strings.TrimSpace(targetUserID)
	role = strings.TrimSpace(strings.ToLower(role))
	if targetUserID == "" {
		return code.New(code.BadRequest, "userId is required")
	}
	if role != "owner" && role != "admin" && role != "member" {
		return code.New(code.BadRequest, "role must be owner/admin/member")
	}

	now := time.Now()
	member := &dbmodel.WorkspaceMember{WorkspaceID: workspaceID, UserID: targetUserID, Role: role, Status: true, JoinedAt: now, CreatedAt: now, UpdatedAt: now}
	// 使用 FirstOrCreate + Assign 实现“存在则更新，不存在则创建”的幂等效果。
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

	result := s.db.WithContext(ctx).Model(&dbmodel.WorkspaceMember{}).
		Where("workspace_id = ? and user_id = ?", workspaceID, targetUserID).
		Updates(map[string]any{"status": false, "updated_at": time.Now()})
	if result.Error != nil {
		return fmt.Errorf("remove workspace member failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return code.New(code.NotFound, "member not found")
	}
	return nil
}

// assertWorkspaceMember 校验用户是否属于目标工作空间。
func (s *UserService) assertWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&dbmodel.WorkspaceMember{}).Where("workspace_id = ? and user_id = ? and status = 1", workspaceID, userID).Count(&count).Error; err != nil {
		return fmt.Errorf("query workspace member failed: %w", err)
	}
	if count == 0 {
		return code.New(code.NoPermission, "workspace not accessible")
	}
	return nil
}

// assertWorkspaceManagePermission 校验工作空间管理权限（owner/admin）。
func (s *UserService) assertWorkspaceManagePermission(ctx context.Context, workspaceID, userID string) error {
	type row struct{ Role string }
	var r row
	err := s.db.WithContext(ctx).Model(&dbmodel.WorkspaceMember{}).
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
