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
	AppName   string            `yaml:"app_name"`
	HTTP      HTTPConfig        `yaml:"http"`
	Database  orm.DBConfig      `yaml:"database"`
	Redis     cache.RedisConfig `yaml:"redis"`
	LLM       LLMConfig         `yaml:"llm"`
	Embedding EmbeddingConfig   `yaml:"embedding"`
}

// HTTPConfig 表示 HTTP 服务配置。
type HTTPConfig struct {
	Addr string `yaml:"addr"`
}

// LLMConfig 是大模型配置。
type LLMConfig struct {
	Provider  string `yaml:"provider"`
	BaseURL   string `yaml:"base_url"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	TimeoutMs int    `yaml:"timeout_ms"`
}

// EmbeddingConfig 是向量模型配置。
type EmbeddingConfig struct {
	Provider  string `yaml:"provider"`
	BaseURL   string `yaml:"base_url"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	TimeoutMs int    `yaml:"timeout_ms"`
	Dims      int    `yaml:"dims"`
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
	if cfg.LLM.TimeoutMs <= 0 {
		cfg.LLM.TimeoutMs = 8000
	}
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if cfg.Embedding.TimeoutMs <= 0 {
		cfg.Embedding.TimeoutMs = 8000
	}
	if cfg.Embedding.Dims <= 0 {
		cfg.Embedding.Dims = 1024
	}
	if v := os.Getenv("EMBEDDING_API_KEY"); v != "" {
		cfg.Embedding.APIKey = v
	}
	return &cfg, nil
}
