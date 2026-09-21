// Package log 日志仓储 GORM 实现：写入走内存队列异步落库，附带每日保留期自动清理。
// 管理端登录已委外给平台（登录日志在平台侧），本仓储只承载操作日志（写请求审计）。
package log

import (
	"context"
	"fmt"
	"sync"
	"time"

	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 写入队列容量：超出即丢弃并告警（日志属于尽力而为的旁路数据，不反压业务主流程）
const queueSize = 256

// Repo 日志仓储（操作日志；写异步、读删同步）
type Repo struct {
	data          *data.Data
	queue         chan *model.OperationLogPO
	done          chan struct{} // 通知保留期清理循环退出
	wg            sync.WaitGroup
	retentionDays int
}

// NewRepo 创建日志仓储：启动异步写入 worker（保留期清理已迁移为定时任务 handler）。
// cleanup 冲刷剩余队列并停止后台协程（关停时先于数据库连接关闭执行）。
func NewRepo(d *data.Data, c *conf.Bootstrap) (*Repo, func(), error) {
	r := &Repo{
		data:          d,
		queue:         make(chan *model.OperationLogPO, queueSize),
		done:          make(chan struct{}),
		retentionDays: c.Log.RetentionDays,
	}
	r.wg.Add(1)
	go r.writeWorker()
	cleanup := func() {
		close(r.done)
		close(r.queue) // worker 消费完剩余条目后退出
		r.wg.Wait()
	}
	return r, cleanup, nil
}

// writeWorker 异步写入协程：逐条落库（管理端写频低，无需攒批），失败告警不重试
func (r *Repo) writeWorker() {
	defer r.wg.Done()
	ctx := context.Background()
	for item := range r.queue {
		if err := r.data.DB.WithContext(ctx).Create(item).Error; err != nil {
			logger.Warn("log write failed", zap.Error(err))
		}
	}
}

// CleanupExpired 清理保留期外日志（定时任务 handler 调用；保留期<=0 永久保留）
func (r *Repo) CleanupExpired(ctx context.Context) error {
	if r.retentionDays <= 0 {
		return nil
	}
	return r.cleanupExpired(ctx, r.retentionDays)
}

// cleanupExpired 物理删除保留期外的日志（与手动清空共用删除路径）
func (r *Repo) cleanupExpired(ctx context.Context, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	n, err := r.DeleteOperationBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("操作日志清理失败: %w", err)
	}
	if n > 0 {
		logger.Info("log retention cleanup", zap.Int64("operation_logs_deleted", n))
	}
	return nil
}

// ---- 写入（异步） ----

func (r *Repo) CreateOperation(o *bizlog.OperationLog) {
	select {
	case r.queue <- model.OperationLogToPO(o):
	default:
		logger.Warn("operation log queue full, dropped", zap.String("route", o.Route))
	}
}

// ---- 查询 ----

func (r *Repo) ListOperationLogs(ctx context.Context, q bizlog.OperationLogQuery, page, pageSize int) ([]*bizlog.OperationLog, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.OperationLogPO{})
	if q.Username != "" {
		tx = tx.Where("username LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Username)+"%")
	}
	if q.Method != "" {
		tx = tx.Where("method = ?", q.Method)
	}
	if q.Keyword != "" {
		// 关键词对动作名/路由模板/实际路径做包含匹配
		kw := "%" + security.EscapeLike(q.Keyword) + "%"
		tx = tx.Where("action LIKE ? ESCAPE '/' OR route LIKE ? ESCAPE '/' OR path LIKE ? ESCAPE '/'", kw, kw, kw)
	}
	if !q.Start.IsZero() {
		tx = tx.Where("created_at >= ?", q.Start)
	}
	if !q.End.IsZero() {
		tx = tx.Where("created_at <= ?", q.End)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.OperationLogPO
	if err := listPage(tx, page, pageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*bizlog.OperationLog, 0, len(pos))
	for i := range pos {
		out = append(out, model.OperationLogFromPO(&pos[i]))
	}
	return out, total, nil
}

// listPage 统一排序与分页（page_size=0 为全量）
func listPage(tx *gorm.DB, page, pageSize int) *gorm.DB {
	tx = tx.Order("id DESC")
	if pageSize > 0 {
		tx = tx.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	return tx
}

// ---- 删除（手动清空与保留期清理共用） ----

func (r *Repo) DeleteOperationBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.data.DB.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&model.OperationLogPO{})
	return res.RowsAffected, res.Error
}
