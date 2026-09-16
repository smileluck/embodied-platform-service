package data

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/cache"
)

// RBACCache 准入/授权决策缓存（wire 单例：RBAC 中间件与 admission/role 用例共享同一实例，
// 变更时 Flush 即时生效——即「禁用/删除/改角色」对已登录请求的同步吊销）
type RBACCache struct {
	*cache.TwoLevel
}

// NewRBACCache 构造（L1 30s 进程内存 + L2 60s Redis）
func NewRBACCache(rdb *redis.Client, cfg *conf.Bootstrap) *RBACCache {
	return &RBACCache{cache.NewTwoLevel(rdb, "rbac:", 30*time.Second, 60*time.Second, cfg.Cache.L2Enabled)}
}

// Flush 实现 biz 侧 DecisionCache 接口
func (c *RBACCache) Flush(ctx context.Context) {
	c.TwoLevel.Flush(ctx)
}
