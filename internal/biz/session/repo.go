package session

import (
	"context"
	"time"
)

// Repo 会话仓储接口（由 data 层 Redis 实现实现，依赖倒置）
type Repo interface {
	// Save 写入会话（设置 TTL），同时维护全局/用户索引与同端互斥键；
	// 同用户同端已存在旧会话时一并吊销（同端互斥在仓储层原子完成）
	Save(ctx context.Context, s *Session, ttl time.Duration) error
	// Find 按 sid 读取会话，不存在返回 ErrSessionNotFound
	Find(ctx context.Context, sid string) (*Session, error)
	// ListAll 全量在线会话（按全局索引读取，惰性清理已过期条目）
	ListAll(ctx context.Context) ([]*Session, error)
	// Extend 续期会话 TTL 并刷新活跃时间（refresh 轮转时调用）
	Extend(ctx context.Context, sid string, ttl time.Duration) error
	// Touch 仅刷新最近活跃时间
	Touch(ctx context.Context, sid string) error
	// TouchIfDue 节流刷新：距上次 touch 未超过 interval 则跳过（节流状态存 Redis，多实例共享）
	TouchIfDue(ctx context.Context, sid string, interval time.Duration) error
	// Revoke 吊销单个会话（清理主体键与全部索引）
	Revoke(ctx context.Context, sid string) error
	// FindSidsByUser 用户全部会话 ID（已过期条目由调用方过滤）
	FindSidsByUser(ctx context.Context, userID uint) ([]string, error)
}
