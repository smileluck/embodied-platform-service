// Package data —— Redis 连接工厂（会话状态存储）。
package data

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"go.uber.org/zap"
)

// NewRedisClient 创建 Redis 客户端（会话状态存储；连接失败 fail-fast，认证随之关闭）
func NewRedisClient(c *conf.Bootstrap) (*redis.Client, func(), error) {
	client := redis.NewClient(&redis.Options{
		Addr:     c.Redis.Addr,
		Password: c.Redis.Password,
		DB:       c.Redis.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	logger.Info("redis connected", zap.String("addr", c.Redis.Addr), zap.Int("db", c.Redis.DB))
	cleanup := func() { _ = client.Close() }
	return client, cleanup, nil
}
