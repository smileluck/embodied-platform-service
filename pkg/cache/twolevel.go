// Package cache 二级缓存：L1 进程内存 + L2 Redis。
//
// 读路径 L1 → L2 → singleflight 回源（Load 的 loader）→ 回填两级；
// 也保留 Get/Set 由调用方自行回源回填的用法。
// L2 可通过开关关闭（l2Enabled=false 时退化为纯 L1，多实例间不共享）。
// 定位为短 TTL 读缓存：不保证强一致，变更一致性由 TTL 兜底。
// 写入 TTL 带 ±10% 随机抖动，避免批量同刻过期引发雪崩。
// Flush 提供前缀级清理，作为将来多实例 Redis Pub/Sub 失效广播的落点。
package cache

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// l1MaxEntries L1 条目数硬上限，超过时先清理过期条目，仍超限则随机淘汰一批
const l1MaxEntries = 4096

type l1Entry struct {
	val      string
	expireAt time.Time
}

// TwoLevel L1（内存）+ L2（Redis）二级缓存
type TwoLevel struct {
	rdb    *redis.Client // nil 表示 L2 关闭
	prefix string        // Redis key 前缀
	l1TTL  time.Duration
	l2TTL  time.Duration

	mu sync.RWMutex
	l1 map[string]l1Entry

	sf singleflight.Group
}

// NewTwoLevel 构造二级缓存；l2Enabled=false 或 rdb=nil 时仅启用 L1
func NewTwoLevel(rdb *redis.Client, prefix string, l1TTL, l2TTL time.Duration, l2Enabled bool) *TwoLevel {
	if !l2Enabled {
		rdb = nil
	}
	return &TwoLevel{rdb: rdb, prefix: prefix, l1TTL: l1TTL, l2TTL: l2TTL, l1: map[string]l1Entry{}}
}

// Get 读取缓存；L2 命中时回填 L1
func (c *TwoLevel) Get(ctx context.Context, key string) (string, bool) {
	c.mu.RLock()
	e, ok := c.l1[key]
	c.mu.RUnlock()
	if ok {
		if time.Now().Before(e.expireAt) {
			return e.val, true
		}
		c.mu.Lock()
		delete(c.l1, key)
		c.mu.Unlock()
	}
	if c.rdb == nil {
		return "", false
	}
	val, err := c.rdb.Get(ctx, c.prefix+key).Result()
	if err != nil {
		if err != redis.Nil {
			logger.Warn("l2 cache get failed", zap.String("key", key), zap.Error(err))
		}
		return "", false
	}
	c.setL1(key, val)
	return val, true
}

// Load 读缓存，miss 时用 singleflight 合并并发回源（loader）并回填两级；
// loader 出错时不回填，所有并发等待者收到同一错误。
func (c *TwoLevel) Load(ctx context.Context, key string, loader func(ctx context.Context) (string, error)) (string, error) {
	if val, ok := c.Get(ctx, key); ok {
		return val, nil
	}
	v, err, _ := c.sf.Do(c.prefix+key, func() (any, error) {
		// 等待期间可能已被其他请求回填，再查一次避免重复回源
		if val, ok := c.Get(ctx, key); ok {
			return val, nil
		}
		val, err := loader(ctx)
		if err != nil {
			return "", err
		}
		c.Set(ctx, key, val)
		return val, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// Set 写入两级缓存（L2 失败仅记日志，L1 仍生效）；TTL 带 ±10% 抖动
func (c *TwoLevel) Set(ctx context.Context, key, val string) {
	c.setL1(key, val)
	if c.rdb == nil {
		return
	}
	if err := c.rdb.Set(ctx, c.prefix+key, val, jitter(c.l2TTL)).Err(); err != nil {
		logger.Warn("l2 cache set failed", zap.String("key", key), zap.Error(err))
	}
}

// Del 双删（变更主动失效用）
func (c *TwoLevel) Del(ctx context.Context, key string) {
	c.mu.Lock()
	delete(c.l1, key)
	c.mu.Unlock()
	if c.rdb == nil {
		return
	}
	if err := c.rdb.Del(ctx, c.prefix+key).Err(); err != nil {
		logger.Warn("l2 cache del failed", zap.String("key", key), zap.Error(err))
	}
}

// Flush 前缀级清理：清空整个 L1，并用 SCAN 删除 L2 中该前缀的全部 key。
// 预留给多实例 Redis Pub/Sub 失效广播使用。
func (c *TwoLevel) Flush(ctx context.Context) {
	c.mu.Lock()
	c.l1 = map[string]l1Entry{}
	c.mu.Unlock()
	if c.rdb == nil {
		return
	}
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, c.prefix+"*", 100).Result()
		if err != nil {
			logger.Warn("l2 cache flush scan failed", zap.String("prefix", c.prefix), zap.Error(err))
			return
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				logger.Warn("l2 cache flush del failed", zap.String("prefix", c.prefix), zap.Error(err))
			}
		}
		if next == 0 {
			return
		}
		cursor = next
	}
}

func (c *TwoLevel) setL1(key, val string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.l1) >= l1MaxEntries {
		now := time.Now()
		for k, e := range c.l1 {
			if now.After(e.expireAt) {
				delete(c.l1, k)
			}
		}
		// 清理过期项后仍超限：随机淘汰约 1/8（Go map 迭代顺序随机）
		if len(c.l1) >= l1MaxEntries {
			n := l1MaxEntries / 8
			for k := range c.l1 {
				delete(c.l1, k)
				if n--; n <= 0 {
					break
				}
			}
		}
	}
	c.l1[key] = l1Entry{val: val, expireAt: time.Now().Add(jitter(c.l1TTL))}
}

// jitter 返回带 ±10% 随机抖动的 TTL
func jitter(d time.Duration) time.Duration {
	delta := int64(d) / 10
	if delta <= 0 {
		return d
	}
	return d + time.Duration(rand.Int64N(2*delta+1)-delta)
}
