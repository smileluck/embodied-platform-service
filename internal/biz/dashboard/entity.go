// Package dashboard 仪表盘限界上下文 —— 领域层。
// 跨上下文只读聚合（计数/活跃趋势/最近操作），供首页展示。
// 登录在平台侧，本地无登录日志：趋势与最近记录以操作日志为口径。
package dashboard

import "time"

// DailyPoint 按日聚合点
type DailyPoint struct {
	Date    string `json:"date"` // YYYY-MM-DD
	Total   int64  `json:"total"`
	Success int64  `json:"success"` // 登录趋势用；操作趋势恒 0
}

// 登录状态展示值
const (
	LoginSuccess = "success"
	LoginFail    = "fail"
)

// LoginItem 最近登录行
type LoginItem struct {
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	Status    string    `json:"status"` // success | fail
	CreatedAt time.Time `json:"created_at"`
}

// Stats 首页聚合结果
type Stats struct {
	Cards struct {
		Users       int64 `json:"users"`        // 准入用户数（平台身份投影）
		Roles       int64 `json:"roles"`        // 本地角色数
		TodayLogins int64 `json:"today_logins"` // 今日活跃操作用户（本地无登录日志，以操作近似）
	} `json:"cards"`
	LoginTrend   []DailyPoint `json:"login_trend"`   // 近 7 日活跃趋势：total=操作量 success=活跃用户数（含零填充）
	OpTrend      []DailyPoint `json:"op_trend"`      // 近 7 日操作趋势
	RecentLogins []LoginItem  `json:"recent_logins"` // 最近 8 条操作记录
}

// Repo 聚合仓储接口
type Repo interface {
	Counts() (users, roles int64, err error)
	TodayLogins() (int64, error)
	LoginTrend(days int) ([]DailyPoint, error)
	OpTrend(days int) ([]DailyPoint, error)
	RecentLogins(n int) ([]LoginItem, error)
}
