// Package blacklist 黑名单限界上下文 —— 领域层
package blacklist

import "time"

// 哨兵错误
var (
	ErrInvalidIP     = errorString("invalid IP address")
	ErrIPExists      = errorString("IP already exists in blacklist")
	ErrNotFound      = errorString("blacklist entry not found")
	ErrSelfBan       = errorString("cannot ban your own IP")
	ErrInvalidExpire = errorString("expire time must be in the future")
)

// 封禁来源
const (
	SourceManual = "manual" // 管理员手工维护
	SourceAuto   = "auto"   // 登录连续失败自动封禁（临时）
)

// AutoBanReason 自动封禁记录的封禁原因
const AutoBanReason = "登录连续失败自动封禁"

// 登录防护参数（临时封禁 + 限流，状态存 Redis）
const (
	LoginFailThreshold = 5                // 计数窗口内失败次数阈值
	LoginFailWindow    = 10 * time.Minute // 失败计数窗口
	TempBanDuration    = 10 * time.Minute // 临时封禁时长
	LoginRateWindow    = time.Minute      // 登录限流窗口
	LoginRateMax       = 5                // 窗口内最大登录尝试次数
)

type errorString string

func (e errorString) Error() string { return string(e) }

// IPBlacklist IP 黑名单实体
type IPBlacklist struct {
	ID          uint
	IP          string
	Reason      string
	Source      string     // manual | auto
	ExpireAt    *time.Time // nil 表示永久
	CreatorID   uint
	CreatorName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Query 列表查询条件（零值为不限）
type Query struct {
	IP string // IP 前缀模糊
}
