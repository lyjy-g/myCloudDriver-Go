package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	"myclouddrive-go/internal/plugin"
	"myclouddrive-go/internal/plugin/boot"
	dbmodel "myclouddrive-go/internal/storage/model/dbmodel"
	modelgen "myclouddrive-go/internal/storage/model/gen"
)

var (
	runtimeOnce sync.Once
	runtimeInst *Runtime
)

// PutOptions 是存储写入选项（业务层可感知字段）。
type PutOptions struct {
	ContentType   string
	ContentLength *int64
	Metadata      map[string]string
}

// ObjectInfo 是对象元信息（屏蔽底层插件细节）。
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified *time.Time
}

// Runtime 负责将“工作空间配置”解析为具体 Store，并提供统一对象操作能力。
type Runtime struct {
	db                  *gorm.DB
	manager             *boot.Manager
	workspaceIDResolver func(context.Context) string
}

// NewRuntime 创建插件运行时服务。
func NewRuntime(db *gorm.DB, manager *boot.Manager, workspaceIDResolver func(context.Context) string) *Runtime {
	return &Runtime{
		db:                  db,
		manager:             manager,
		workspaceIDResolver: workspaceIDResolver,
	}
}

// Init 初始化插件运行时单例。
func Init(db *gorm.DB) {
	runtimeOnce.Do(func() {
		runtimeInst = NewRuntime(db, boot.NewDefaultManager(), currentWorkspaceID)
	})
}

// GetRuntime 返回插件运行时实例；若未初始化会按需初始化。
func GetRuntime(db *gorm.DB) *Runtime {
	Init(db)
	return runtimeInst
}

// Invalidate 主动失效指定配置对应的缓存实例。
func (r *Runtime) Invalidate(settingID string) {
	r.manager.Invalidate(settingID)
}

// Put 使用当前工作空间激活的配置写入对象。
func (r *Runtime) Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) (ObjectInfo, error) {
	var out ObjectInfo
	err := r.withActiveStore(ctx, func(store plugin.Store) error {
		info, err := store.Put(ctx, plugin.Key(key), reader, plugin.PutOptions{
			ContentType:   opts.ContentType,
			ContentLength: opts.ContentLength,
			Metadata:      opts.Metadata,
		})
		if err != nil {
			return err
		}
		out = toRuntimeObjectInfo(info)
		return nil
	})
	return out, err
}

// Get 使用当前工作空间激活的配置读取对象。
func (r *Runtime) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	var (
		body io.ReadCloser
		out  ObjectInfo
	)
	err := r.withActiveStore(ctx, func(store plugin.Store) error {
		rc, info, err := store.Get(ctx, plugin.Key(key))
		if err != nil {
			return err
		}
		body = rc
		out = toRuntimeObjectInfo(info)
		return nil
	})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return body, out, nil
}

// Delete 使用当前工作空间激活的配置删除对象。
func (r *Runtime) Delete(ctx context.Context, key string) error {
	return r.withActiveStore(ctx, func(store plugin.Store) error {
		return store.Delete(ctx, plugin.Key(key))
	})
}

// Stat 使用当前工作空间激活的配置查询对象元信息。
func (r *Runtime) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	var out ObjectInfo
	err := r.withActiveStore(ctx, func(store plugin.Store) error {
		info, err := store.Stat(ctx, plugin.Key(key))
		if err != nil {
			return err
		}
		out = toRuntimeObjectInfo(info)
		return nil
	})
	return out, err
}

// PresignGet 使用当前工作空间激活配置生成下载预签名地址。
func (r *Runtime) PresignGet(ctx context.Context, key string, expire time.Duration) (string, error) {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return "", err
	}
	signed, ok := store.(plugin.SignedURLStore)
	if !ok {
		return "", fmt.Errorf("%w: presign get", code.ErrCapabilityNotMatch)
	}
	return signed.PresignGet(ctx, plugin.Key(key), expire)
}

// PresignPut 使用当前工作空间激活配置生成上传预签名地址。
func (r *Runtime) PresignPut(ctx context.Context, key string, expire time.Duration) (string, error) {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return "", err
	}
	signed, ok := store.(plugin.SignedURLStore)
	if !ok {
		return "", fmt.Errorf("%w: presign put", code.ErrCapabilityNotMatch)
	}
	return signed.PresignPut(ctx, plugin.Key(key), expire)
}

func (r *Runtime) resolveActiveStore(ctx context.Context) (plugin.Store, error) {
	row, err := r.getWorkspaceActiveSetting(ctx)
	if err != nil {
		return nil, err
	}
	return r.resolveStoreBySetting(ctx, row)
}

// withActiveStore 统一处理“获取当前激活 store + 执行业务动作”流程，减少重复代码。
func (r *Runtime) withActiveStore(ctx context.Context, fn func(store plugin.Store) error) error {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return err
	}
	return fn(store)
}

func (r *Runtime) getWorkspaceActiveSetting(ctx context.Context) (dbmodel.StorageSetting, error) {
	workspaceID := r.workspaceIDResolver(ctx)
	q := modelgen.Use(r.db)
	ss := q.StorageSetting

	row, err := q.WithContext(ctx).StorageSetting.
		Where(
			ss.WorkspaceID.Eq(workspaceID),
			ss.Enabled.Is(true),
			ss.Deleted.Is(false),
		).
		Order(ss.UpdatedAt.Desc()).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dbmodel.StorageSetting{}, fmt.Errorf("%w: active setting for workspace %s", code.ErrSettingNotFound, workspaceID)
		}
		return dbmodel.StorageSetting{}, fmt.Errorf("query active storage setting failed: %w", err)
	}
	return *row, nil
}

func (r *Runtime) resolveStoreBySetting(ctx context.Context, row dbmodel.StorageSetting) (plugin.Store, error) {
	cfg := plugin.ResolvedStorageConfig{
		SettingID:          row.ID,
		WorkspaceID:        row.WorkspaceID,
		PlatformIdentifier: plugin.PlatformIdentifier(row.PlatformIdentifier),
		ConfigData:         []byte(row.ConfigData),
		Version:            plugin.Fingerprint(row),
	}

	store, err := r.manager.Resolve(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve storage plugin failed: %w", err)
	}
	return store, nil
}

func toRuntimeObjectInfo(info plugin.ObjectInfo) ObjectInfo {
	return ObjectInfo{
		Key:          string(info.Key),
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		LastModified: info.LastModified,
	}
}

func currentWorkspaceID(ctx context.Context) string {
	if p, ok := security.PrincipalFromContext(ctx); ok && strings.TrimSpace(p.WorkspaceID) != "" {
		return p.WorkspaceID
	}
	return "ws_01jrvgs943q0f43h0aa5mjde0y_personal"
}
