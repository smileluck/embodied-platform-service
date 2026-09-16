package log

import (
	"context"
	"time"
)

// Repo 日志仓储接口（由 data 层实现，依赖倒置）。
// 写入为异步落库（fire-and-forget，不阻塞请求）；查询与删除同步。
type Repo interface {
	// CreateOperation 操作日志入队异步落库
	CreateOperation(o *OperationLog)
	// ListOperationLogs 分页查询操作日志（page_size=0 为全量），返回总数
	ListOperationLogs(ctx context.Context, q OperationLogQuery, page, pageSize int) ([]*OperationLog, int64, error)
	// DeleteOperationBefore 物理删除 cutoff 之前（不含）的操作日志，返回删除行数；
	// 手动清空（DeleteOperationBefore(now)）与保留期自动清理共用本删除路径
	DeleteOperationBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
