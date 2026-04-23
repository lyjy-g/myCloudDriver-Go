package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
)

const (
	defaultTokenTTL      = 24 * time.Hour
	rememberTokenTTL     = 7 * 24 * time.Hour
	defaultForgotCodeTTL = 10 * time.Minute
	defaultChunkSize     = int64(5 * 1024 * 1024)
)

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

// Ping 服务健康检查。
func (s *UserService) Ping(_ context.Context) (string, error) {
	return "user service ready", nil
}

func hashPassword(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

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

func randomID(prefix string) string {
	if strings.TrimSpace(prefix) == "" {
		prefix = "id"
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func requirePrincipal(ctx context.Context) (security.Principal, error) {
	p, ok := security.PrincipalFromContext(ctx)
	if !ok || strings.TrimSpace(p.UserID) == "" {
		return security.Principal{}, code.New(code.NoPermission, "login required")
	}
	return p, nil
}

func resolveCurrentUserID(ctx context.Context) (string, error) {
	p, err := requirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	return p.UserID, nil
}

func resolveCurrentWorkspaceID(ctx context.Context) string {
	if p, ok := security.PrincipalFromContext(ctx); ok && strings.TrimSpace(p.WorkspaceID) != "" {
		return p.WorkspaceID
	}
	return ""
}

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

func assertPasswordPair(password, confirm string) error {
	if strings.TrimSpace(password) == "" {
		return code.New(code.BadRequest, "password is required")
	}
	if password != confirm {
		return code.New(code.BadRequest, "password and confirmPassword not match")
	}
	return nil
}

func (s *UserService) mustInitTransferSetting(ctx context.Context, userID string) error {
	var item SysUserTransferSetting
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&item).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query transfer setting failed: %w", err)
	}

	now := time.Now()
	create := &SysUserTransferSetting{
		UserID:                     userID,
		DownloadLocation:           "",
		IsDefaultDownloadLocation:  0,
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

func forgetCodeCacheKey(mail string) string {
	return "user:forget:code:" + strings.ToLower(strings.TrimSpace(mail))
}
