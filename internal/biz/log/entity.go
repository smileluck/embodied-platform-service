// Package log 日志限界上下文 —— 领域层。
// 管理端登录已委外给平台（登录日志在平台侧），本上下文只承载操作日志：
// 审计全部写请求，追加型流水，清空与保留期清理共用同一条按时间物理删除的路径。
package log

import "time"

// OperationLog 操作日志（写请求审计）
type OperationLog struct {
	ID         uint
	UserID     uint   // 操作人（平台用户 ID；认证被拒时为 0）
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
	Username string // 操作人前缀模糊
	Method   string // 请求方式精确匹配（空为全部）
	Keyword  string // 动作/路由/路径包含匹配
	Start    time.Time
	End      time.Time
}
