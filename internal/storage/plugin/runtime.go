package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"myclouddrive-go/internal/storage/model/dbmodel"
	"time"
)

// ResolvedStorageConfig 表示已解析完成的运行时存储配置。
type ResolvedStorageConfig struct {
	SettingID          string
	UserID             string
	PlatformIdentifier PlatformIdentifier
	ConfigData         []byte
	Version            string
}

// Fingerprint 基于配置核心字段生成实例版本指纹。
func Fingerprint(setting dbmodel.StorageSetting) string {
	h := sha256.New()
	h.Write([]byte(string(setting.PlatformIdentifier)))
	h.Write([]byte("|"))
	h.Write([]byte(setting.UpdatedAt.UTC().Format(time.RFC3339Nano)))
	h.Write([]byte("|"))
	h.Write([]byte(setting.ConfigData))
	return hex.EncodeToString(h.Sum(nil))
}
