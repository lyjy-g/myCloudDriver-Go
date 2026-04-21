package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig 是 Redis 连接配置。
type RedisConfig struct {
	Username string `yaml:"username"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Required bool   `yaml:"required"`
}

// NewRedisClient 初始化 go-redis 客户端并做 ping 检查。
func NewRedisClient(cfg RedisConfig) (client *redis.Client, err error) {
	client = redis.NewClient(&redis.Options{
		Username: cfg.Username,
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 定义一个闭包：如果函数返回了错误，就关闭已经创建的资源
	defer func() {
		if err != nil && client != nil {
			err := client.Close()
			if err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err = client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return client, nil
}
