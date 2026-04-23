package local

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/storage/plugin"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config 是本地存储插件配置。
type Config struct {
	// BasePath 为本地文件根目录，例如 /data/myclouddrive。
	BasePath string `json:"base_path"`
}

// Driver 是本地存储驱动实现。
type Driver struct{}

// NewDriver 创建本地存储驱动。
func NewDriver() *Driver {
	return &Driver{}
}

// PlatformIdentifier 返回驱动标识。
func (d *Driver) PlatformIdentifier() plugin.PlatformIdentifier {
	return plugin.PlatformIdentifierLocal
}

// Capabilities 返回驱动能力集合。
func (d *Driver) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityBasic}
}

// ValidateConfig 校验配置是否合法。
func (d *Driver) ValidateConfig(_ context.Context, raw []byte) error {
	cfg, err := parseConfig(raw)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.BasePath) == "" {
		return fmt.Errorf("local config.base_path is required")
	}
	return nil
}

// Build 基于配置构建 Store。
func (d *Driver) Build(_ context.Context, cfg plugin.ResolvedStorageConfig) (plugin.Store, error) {
	parsed, err := parseConfig(cfg.ConfigData)
	if err != nil {
		return nil, err
	}

	basePath := filepath.Clean(parsed.BasePath)
	if basePath == "." || strings.TrimSpace(basePath) == "" {
		return nil, fmt.Errorf("invalid local base_path")
	}
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("create local base_path failed: %w", err)
	}

	return &store{basePath: basePath}, nil
}

// store 是本地文件系统存储实现。
type store struct {
	basePath string
}

func (s *store) Put(_ context.Context, key plugin.Key, r io.Reader, opts plugin.PutOptions) (plugin.ObjectInfo, error) {
	path, normalized, err := s.keyPath(key)
	if err != nil {
		return plugin.ObjectInfo{}, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return plugin.ObjectInfo{}, fmt.Errorf("create local parent dir failed: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return plugin.ObjectInfo{}, fmt.Errorf("create local object failed: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := md5.New()
	size, err := io.Copy(io.MultiWriter(f, h), r)
	if err != nil {
		return plugin.ObjectInfo{}, fmt.Errorf("write local object failed: %w", err)
	}

	now := time.Now()
	contentType := opts.ContentType
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(normalized))
	}

	return plugin.ObjectInfo{
		Key:          plugin.Key(normalized),
		Size:         size,
		ContentType:  contentType,
		ETag:         hex.EncodeToString(h.Sum(nil)),
		LastModified: &now,
	}, nil
}

func (s *store) Get(_ context.Context, key plugin.Key) (io.ReadCloser, plugin.ObjectInfo, error) {
	path, normalized, err := s.keyPath(key)
	if err != nil {
		return nil, plugin.ObjectInfo{}, err
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, plugin.ObjectInfo{}, fmt.Errorf("%w: %s", code.ErrObjectNotFound, normalized)
		}
		return nil, plugin.ObjectInfo{}, fmt.Errorf("open local object failed: %w", err)
	}

	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, plugin.ObjectInfo{}, fmt.Errorf("stat local object failed: %w", err)
	}

	mod := stat.ModTime()
	info := plugin.ObjectInfo{
		Key:          plugin.Key(normalized),
		Size:         stat.Size(),
		ContentType:  mime.TypeByExtension(filepath.Ext(normalized)),
		LastModified: &mod,
	}

	return f, info, nil
}

func (s *store) Delete(_ context.Context, key plugin.Key) error {
	path, normalized, err := s.keyPath(key)
	if err != nil {
		return err
	}
	if err = os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", code.ErrObjectNotFound, normalized)
		}
		return fmt.Errorf("delete local object failed: %w", err)
	}
	return nil
}

func (s *store) Stat(_ context.Context, key plugin.Key) (plugin.ObjectInfo, error) {
	path, normalized, err := s.keyPath(key)
	if err != nil {
		return plugin.ObjectInfo{}, err
	}

	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return plugin.ObjectInfo{}, fmt.Errorf("%w: %s", code.ErrObjectNotFound, normalized)
		}
		return plugin.ObjectInfo{}, fmt.Errorf("stat local object failed: %w", err)
	}
	mod := stat.ModTime()

	return plugin.ObjectInfo{
		Key:          plugin.Key(normalized),
		Size:         stat.Size(),
		ContentType:  mime.TypeByExtension(filepath.Ext(normalized)),
		LastModified: &mod,
	}, nil
}

// keyPath 将对象 key 解析为安全的本地路径，防止目录穿越。
func (s *store) keyPath(key plugin.Key) (string, string, error) {
	raw := strings.TrimSpace(string(key))
	if raw == "" {
		return "", "", code.ErrInvalidKey
	}

	normalized := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+raw)), "/")
	if normalized == "." || normalized == "" {
		return "", "", code.ErrInvalidKey
	}

	fullPath := filepath.Join(s.basePath, filepath.FromSlash(normalized))
	rel, err := filepath.Rel(s.basePath, fullPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve local key path failed: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", code.ErrInvalidKey
	}

	return fullPath, normalized, nil
}

func parseConfig(raw []byte) (Config, error) {
	var cfg Config
	if len(raw) == 0 {
		return cfg, fmt.Errorf("local storage config is empty")
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse local config failed: %w", err)
	}
	return cfg, nil
}
