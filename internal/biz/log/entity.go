// Package log 日志限界上下文 —— 领域层。
// 登录日志记录每次登录尝试（成功/失败/IP 被封禁拦截），操作日志审计全部写请求；
// 两者均为追加型流水，清空与保留期清理共用同一条按时间物理删除的路径。
package log

import "time"

// 登录结果
const (
	LoginStatusSuccess = 1
	LoginStatusFail    = 0
)

// LoginLog 登录日志
type LoginLog struct {
	ID        uint
	Username  string // 尝试登录的用户名（可能不存在）
	IP        string
	UserAgent string
	Device    string // web / app
	Status    int    // 1 成功 0 失败
	Msg       string // 失败原因（成功为空）
	CreatedAt time.Time
}

// LoginLogQuery 登录日志查询条件（零值为不限）
type LoginLogQuery struct {
	Username string    // 用户名前缀模糊
	IP       string    // IP 精确匹配
	Status   *int      // 1 成功 0 失败（nil 为全部）
	Start    time.Time // 登录时间范围起（含）
	End      time.Time // 登录时间范围止（含）
}

// OperationLog 操作日志（写请求审计）
type OperationLog struct {
	ID         uint
	UserID     uint   // 操作人（JWT 被拒时为 0）
	Username   string // 操作人用户名快照
	Method     string // POST / PUT / DELETE / PATCH
	Path       string // 实际请求路径（含资源 ID 与 query）
	Route      string // 路由模板（如 /api/v1/users/:id）
	Action     string // 中文动作名（如「新增用户」）
	Params     string // 请求参数摘要（敏感字段脱敏、超长截断）
	IP         string
	UserAgent  string
	StatusCode int // 响应状态码
	LatencyMs  int // 耗时（毫秒）
	CreatedAt  time.Time
}

// OperationLogQuery 操作日志查询条件（零值为不限）
type OperationLogQuery struct {
	Username string    // 操作人前缀模糊
	Method   string    // 请求方式精确匹配（空为全部）
	Keyword  string    // 动作/路由/路径包含匹配
	Start    time.Time
	End      time.Time
}
