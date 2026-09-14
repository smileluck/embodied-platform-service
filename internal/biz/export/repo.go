package export

import (
	"context"
	"time"
)

// Repo 导出记录仓储接口（由 data 层实现，依赖倒置）。
// 记录无软删：保留期清理与手动删除均为物理删除。
type Repo interface {
	Create(ctx context.Context, r *ExportRecord) error
	// Update 全量更新（worker 状态推进与结果落库共用）
	Update(ctx context.Context, r *ExportRecord) error
	// ListByUser 按归属用户分页查询（按创建时间倒序），返回总数
	ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]*ExportRecord, int64, error)
	// RecentByUser 按归属用户取最近 limit 条（任务浮层轮询用）
	RecentByUser(ctx context.Context, userID uint, limit int) ([]*ExportRecord, error)
	FindByID(ctx context.Context, id uint) (*ExportRecord, error)
	Delete(ctx context.Context, id uint) error
	// DeleteBefore 物理删除 cutoff 之前（不含）创建的记录，返回删除行数（保留期清理用）
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
