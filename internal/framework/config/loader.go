package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"myclouddrive-go/internal/framework/cache"
	"myclouddrive-go/internal/framework/orm"
)

// Config 表示应用配置模型。
type Config struct {
	AppName  string            `yaml:"app_name"`
	HTTP     HTTPConfig        `yaml:"http"`
	Database orm.DBConfig      `yaml:"database"`
	Redis    cache.RedisConfig `yaml:"redis"`
}

// HTTPConfig 表示 HTTP 服务配置。
type HTTPConfig struct {
	Addr string `yaml:"addr"`
}

// Load 从 yaml 文件加载配置。
func Load(path string) (*Config, error) {
	//读取文件到data
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config

	//读取data为cfg类型
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if cfg.HTTP.Addr == "" {
		cfg.HTTP.Addr = ":8080"
	}
	return &cfg, nil
}
