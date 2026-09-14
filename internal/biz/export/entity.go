// Package export 异步导出限界上下文 —— 领域层。
// 提交导出任务落 pending 记录并入队，后台 worker 分批拉数写 CSV 经存储后端落盘，
// 产物与记录按保留期自动清理；记录归属用户，仅本人可见/下载/删除。
package export

import (
	"errors"
	"time"
)

// 导出任务状态
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// ErrQueueFull 导出队列已满（提交方映射为 429）
var ErrQueueFull = errors.New("导出任务队列已满，请稍后重试")

// ErrUnsupportedBiz 不支持的导出业务类型
var ErrUnsupportedBiz = errors.New("不支持的导出类型")

// ErrNotFound 导出记录不存在
var ErrNotFound = errors.New("导出记录不存在")

// ErrNotOwner 记录归属其他用户（下载/删除越权）
var ErrNotOwner = errors.New("无权操作他人的导出记录")

// ErrNotReady 任务未完成（仅 done 状态可下载）
var ErrNotReady = errors.New("导出任务尚未完成，无法下载")

// ExportRecord 导出任务记录（产物本体在 Driver 对应的存储后端，与文件管理同一套存储抽象）
type ExportRecord struct {
	ID         uint
	UserID     uint   // 任务归属用户（列表/下载/删除均强制按此过滤）
	Biz        string // 业务类型（user / login_log / op_log）
	Name       string // 展示名（兼作下载文件名，如 用户列表-20260822103000.csv）
	Params     string // 查询条件快照（url.Values 的 JSON 序列化）
	Driver     string // 产物落库时的存储后端（local | oss | cos | tos | minio）
	ObjectKey  string // 产物对象 key
	Size       int64  // 产物字节数（含 BOM）
	Rows       int    // 已导出的数据行数（不含表头）
	Status     string // pending | running | done | failed
	Truncated  bool   // 触及大小/行数上限被截断
	Error      string // 失败原因（成功为空）
	CreatedAt  time.Time
	FinishedAt *time.Time // 完成/失败时间（未结束为 nil）
}
