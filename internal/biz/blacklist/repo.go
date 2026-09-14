package blacklist

import (
	"context"
	"time"
)

// Repo 黑名单仓储接口（由 data 层实现，依赖倒置；内部含 Redis 读缓存）
type Repo interface {
	// Create 新增黑名单记录
	Create(ctx context.Context, b *IPBlacklist) error
	// Delete 按 ID 删除（解封）
	Delete(ctx context.Context, id uint) error
	// Get 按 ID 查询
	Get(ctx context.Context, id uint) (*IPBlacklist, error)
	// List 分页查询（page_size=0 全量）
	List(ctx context.Context, q Query, page, pageSize int) ([]*IPBlacklist, int64, error)
	// ListActive 返回当前生效的手工封禁 IP 列表（未过期且未被软删），供中间件拦截使用；
	// 自动临时封禁不进入全局拦截（仅登录接口拦截），故不包含在内
	ListActive(ctx context.Context) ([]string, error)
	// IsBlocked 判定 IP 是否在生效黑名单中（Redis 缓存优先，未加载时回源 DB 重建）
	IsBlocked(ctx context.Context, ip string) (bool, error)
	// CacheAdd / CacheRemove 写穿缓存（增删记录后同步 Redis 集合）
	CacheAdd(ctx context.Context, ip string) error
	CacheRemove(ctx context.Context, ip string) error
	// UpsertAutoBan 自动封禁落库：已有生效的手工封禁记录时跳过不覆盖；
	// 已有自动记录或软删记录时复活并刷新过期时间，否则新建
	UpsertAutoBan(ctx context.Context, b *IPBlacklist) error
}

// LoginProtector 登录防护存储接口（Redis 实现：临时封禁 / 失败计数 / 限流）
type LoginProtector interface {
	// TempBanRemaining 返回临时封禁剩余时间；未封禁返回 (0, nil)
	TempBanRemaining(ctx context.Context, ip string) (time.Duration, error)
	// TempBan 临时封禁指定时长
	TempBan(ctx context.Context, ip string, dur time.Duration) error
	// ClearTempBan 提前解除临时封禁
	ClearTempBan(ctx context.Context, ip string) error
	// IncrFail 登录失败计数 +1（首次设置窗口 TTL），返回当前计数
	IncrFail(ctx context.Context, ip string, window time.Duration) (int64, error)
	// ResetFail 清空失败计数
	ResetFail(ctx context.Context, ip string) error
	// IncrRate 登录限流计数 +1（固定窗口），返回窗口内当前次数
	IncrRate(ctx context.Context, ip string, window time.Duration) (int64, error)
	// ResetRate 清空限流计数（解封时联动清理）
	ResetRate(ctx context.Context, ip string) error
}
