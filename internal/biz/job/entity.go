// Package job 定时任务限界上下文 —— 领域层。
// cron 调度 + 内置 handler 注册表（保留期清理等）；单机版语义（无分布式锁），
// 多实例部署时同一任务会各跑一次（清理类任务幂等，可接受）。
package job

import (
	"context"
	"errors"
	"time"
)

// 哨兵错误
var (
	ErrNotFound       = errors.New("任务不存在")
	ErrNameExists     = errors.New("任务名称已存在，请更换")
	ErrBadCron        = errors.New("cron 表达式不合法（5 段：分 时 日 月 周）")
	ErrUnknownHandler = errors.New("任务处理器不存在，请重新选择")
	ErrDisabled       = errors.New("任务已停用")
)

// Status 任务状态
type Status int

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

// Job 定时任务
type Job struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"` // ≤20
	Cron       string     `json:"cron"` // 5 段标准表达式
	HandlerKey string     `json:"handler_key"`
	Params     string     `json:"params"` // 透传给 handler 的参数（JSON 文本，可为空）
	Remark     string     `json:"remark"`
	Status     Status     `json:"status"`
	LastRunAt  *time.Time `json:"last_run_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// JobLog 执行记录（追加流水）
type JobLog struct {
	ID         uint      `json:"id"`
	JobID      uint      `json:"job_id"`
	JobName    string    `json:"job_name"` // 冗余（任务删除后仍可读）
	HandlerKey string    `json:"handler_key"`
	Status     string    `json:"status"` // success | failed
	Output     string    `json:"output"` // 摘要/错误（截断）
	DurationMs int64     `json:"duration_ms"`
	StartedAt  time.Time `json:"started_at"`
}

// 执行状态
const (
	RunSuccess = "success"
	RunFailed  = "failed"
)

// Handler 清理类内置处理器（保留期参数由各仓储自身配置承担，params 备用）
type Handler struct {
	Key         string
	Description string
	Run         func(ctx context.Context, params string) (string, error)
}

// Query 任务列表条件
type Query struct {
	Name string
}

// Repo 仓储接口
type Repo interface {
	Create(ctx context.Context, j *Job) error
	Update(ctx context.Context, j *Job) error
	Delete(ctx context.Context, id uint) error
	Find(ctx context.Context, id uint) (*Job, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*Job, int64, error)
	ListEnabled(ctx context.Context) ([]*Job, error)

	AppendLog(ctx context.Context, l *JobLog) error
	ListLogs(ctx context.Context, jobID uint, page, pageSize int) ([]*JobLog, int64, error)
	CleanupLogsBefore(ctx context.Context, before time.Time) error
}
