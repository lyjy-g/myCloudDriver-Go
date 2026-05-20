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
	syncOnce       sync.Once
	runManagerInst *RunManager
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

// RunManager 负责将“工作空间配置”解析为具体 Store，并提供统一对象操作能力。
type RunManager struct {
	db          *gorm.DB
	manager     *boot.Manager
	workspaceID func(context.Context) string
}

// NewRunManager 创建插件运行时服务。
func NewRunManager(db *gorm.DB, manager *boot.Manager, workspaceIDResolver func(context.Context) string) *RunManager {
	return &RunManager{
		db:          db,
		manager:     manager,
		workspaceID: workspaceIDResolver,
	}
}

// GetRunManager 返回插件运行时实例；若未初始化会按需初始化。
func GetRunManager(db *gorm.DB) *RunManager {
	syncOnce.Do(func() {
		runManagerInst = NewRunManager(db, boot.NewDefaultManager(), currentWorkspaceID)
	})
	return runManagerInst
}

// InitRunStore 兼容模块初始化调用；内部复用 GetRunManager 的懒加载逻辑。
func InitRunStore(db *gorm.DB) {
	_ = GetRunManager(db)
}

// Invalidate 主动失效指定配置对应的缓存实例。
func (r *RunManager) Invalidate(settingID string) {
	r.manager.Invalidate(settingID)
}

// Put 使用当前工作空间激活的配置写入对象。
func (r *RunManager) Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) (ObjectInfo, error) {
	var out ObjectInfo
	err := r.withActiveStore(ctx, func(store plugin.StorePower) error {
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
func (r *RunManager) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	var (
		body io.ReadCloser
		out  ObjectInfo
	)
	err := r.withActiveStore(ctx, func(store plugin.StorePower) error {
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

// GetBySetting 使用指定配置读取对象，避免依赖“当前激活配置”导致串读。
func (r *RunManager) GetBySetting(ctx context.Context, settingID string, key string) (io.ReadCloser, ObjectInfo, error) {
	row, err := r.getSettingByID(ctx, settingID)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	store, err := r.resolveStoreBySetting(ctx, row)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	rc, info, err := store.Get(ctx, plugin.Key(key))
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return rc, toRuntimeObjectInfo(info), nil
}

// Delete 使用当前工作空间激活的配置删除对象。
func (r *RunManager) Delete(ctx context.Context, key string) error {
	return r.withActiveStore(ctx, func(store plugin.StorePower) error {
		return store.Delete(ctx, plugin.Key(key))
	})
}

// Stat 使用当前工作空间激活的配置查询对象元信息。
func (r *RunManager) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	var out ObjectInfo
	err := r.withActiveStore(ctx, func(store plugin.StorePower) error {
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
func (r *RunManager) PresignGet(ctx context.Context, key string, expire time.Duration) (string, error) {
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
func (r *RunManager) PresignPut(ctx context.Context, key string, expire time.Duration) (string, error) {
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

// resolveActiveStore 两级配置选择:
//
//	第一级: context 中若有当前请求解析出的 CurrentStorageSettingID，优先按它查配置。
//	第二级: 没有显式选择时，再回退到 workspace 默认启用配置。
//
// 注意这里切换的只是“配置 ID 路由”，不是切换接口里就提前构建实例。
// 真正的实例加载仍在 resolveStoreBySetting → manager.Resolve 里按需完成。
func (r *RunManager) resolveActiveStore(ctx context.Context) (plugin.StorePower, error) {
	if p, ok := security.GetCtxInfo(ctx); ok {
		if settingID := strings.TrimSpace(p.CurrentStorageSettingID); settingID != "" {
			row, err := r.getSettingByID(ctx, settingID)
			if err == nil {
				return r.resolveStoreBySetting(ctx, row)
			}
		}
	}
	row, err := r.getWorkspaceActiveSetting(ctx)
	if err != nil {
		return nil, err
	}
	return r.resolveStoreBySetting(ctx, row)
}

// withActiveStore 函数参数模式: 先 resolveActiveStore 拿到 store，再把“具体业务动作”作为 fn 传进来执行。
// 这样 Put/Get/Delete/Stat 只关心自己的存储动作，不需要重复写“取当前配置 → 懒加载实例 → 调方法”的模板代码。
func (r *RunManager) withActiveStore(ctx context.Context, fn func(store plugin.StorePower) error) error {
	store, err := r.resolveActiveStore(ctx)
	if err != nil {
		return err
	}
	return fn(store)
}

// getWorkspaceActiveSetting DB查询: 当前 workspace 最新启用的那条配置。
// 缓存不在这一层 — 真正懒加载在 boot.Manager.Resolve 里。
func (r *RunManager) getWorkspaceActiveSetting(ctx context.Context) (dbmodel.StorageSetting, error) {
	workspaceID := r.workspaceID(ctx)
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

// getSettingByID DB查询: 按 settingID 查配置（同时校验 workspace 归属）。
// 用于 context 中指定了 CurrentStorageSettingID 的切换场景。
func (r *RunManager) getSettingByID(ctx context.Context, settingID string) (dbmodel.StorageSetting, error) {
	workspaceID := r.workspaceID(ctx)
	q := modelgen.Use(r.db)
	ss := q.StorageSetting
	row, err := q.WithContext(ctx).StorageSetting.
		Where(
			ss.ID.Eq(strings.TrimSpace(settingID)),
			ss.WorkspaceID.Eq(workspaceID),
			ss.Deleted.Is(false),
		).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dbmodel.StorageSetting{}, fmt.Errorf("%w: setting %s for workspace %s", code.ErrSettingNotFound, settingID, workspaceID)
		}
		return dbmodel.StorageSetting{}, fmt.Errorf("query storage setting failed: %w", err)
	}
	return *row, nil
}

// resolveStoreBySetting 把 DB 配置行封装成 ResolvedStorageConfig，交给 manager.Resolve。
// 这里除了 settingID / platform / configData 外，还会带上 version 指纹。
// Manager 内部直接使用 settingID + version 作为缓存 key，
// 所以同一个 settingID 的配置内容如果被更新，即使没显式 Invalidate，
// 也会因为 version 变化命中不到旧缓存，从而重新构建实例。
//
// manager.Resolve 内部流程:
// 1. 先查内存缓存
// 2. 未命中时用 singleflight 合并并发构建
// 3. 真正执行 Validate + Build
//
// 这就是“懒加载”的真正入口: 只有业务真的用到这个配置时才会触发实例解析。
func (r *RunManager) resolveStoreBySetting(ctx context.Context, row dbmodel.StorageSetting) (plugin.StorePower, error) {
	normalizedIdentifier := strings.ToLower(strings.TrimSpace(row.PlatformIdentifier))
	if normalizedIdentifier == "" {
		return nil, fmt.Errorf("resolve storage plugin failed: empty platform identifier")
	}
	cfg := plugin.ResolvedStorageConfig{
		SettingID:          row.ID,
		WorkspaceID:        row.WorkspaceID,
		PlatformIdentifier: plugin.PlatformIdentifier(normalizedIdentifier),
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
	if p, ok := security.GetCtxInfo(ctx); ok && strings.TrimSpace(p.WorkspaceID) != "" {
		return p.WorkspaceID
	}
	return "ws_01jrvgs943q0f43h0aa5mjde0y_personal"
}
