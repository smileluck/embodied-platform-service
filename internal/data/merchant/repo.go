// Package merchant 商户仓储 GORM 实现：商户同步读写 + API 调用日志异步写入与保留期清理。
package merchant

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	bizmerchant "github.com/smilex/smilex-admin-gin/internal/biz/merchant"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// mapErr 哨兵错误映射：记录不存在 / 唯一索引冲突（sqlite/mysql/postgres 文案判断）
func mapErr(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return bizmerchant.ErrMerchantNotFound
	case isUniqueViolation(err):
		return bizmerchant.ErrDuplicateCode
	}
	return err
}

// isUniqueViolation 各数据库唯一约束冲突文案判断：
// MySQL Error 1062、Postgres 23505（duplicate key value）、SQLite UNIQUE constraint failed
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "UNIQUE constraint failed")
}

// ---- 商户仓储 ----

type repo struct {
	data *data.Data
}

// NewRepo 创建商户仓储
func NewRepo(d *data.Data) bizmerchant.Repo { return &repo{data: d} }

func (r *repo) Create(ctx context.Context, m *bizmerchant.Merchant) error {
	po := model.MerchantToPO(m)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return mapErr(err)
	}
	m.ID = po.ID
	m.CreatedAt, m.UpdatedAt = po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) Update(ctx context.Context, m *bizmerchant.Merchant) error {
	po := model.MerchantToPO(m)
	// code/app_key 创建后不可改；密钥哈希仅重置时变更（置空表示不触碰）
	updates := map[string]interface{}{
		"name": po.Name, "contact_name": po.ContactName, "contact_phone": po.ContactPhone,
		"contact_email": po.ContactEmail, "status": po.Status, "remark": po.Remark,
	}
	if po.AppSecretHash != "" {
		updates["app_secret_hash"] = po.AppSecretHash
	}
	res := r.data.DB.WithContext(ctx).Model(&model.MerchantPO{}).Where("id = ?", m.ID).Updates(updates)
	if res.Error != nil {
		return mapErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return bizmerchant.ErrMerchantNotFound
	}
	return nil
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.MerchantPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return bizmerchant.ErrMerchantNotFound
	}
	return nil
}

func (r *repo) Get(ctx context.Context, id uint) (*bizmerchant.Merchant, error) {
	var po model.MerchantPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.MerchantFromPO(&po), nil
}

// FindByAppKey 按 appKey 查询（开放 API 验签用，携带密钥哈希）
func (r *repo) FindByAppKey(ctx context.Context, appKey string) (*bizmerchant.Merchant, error) {
	var po model.MerchantPO
	if err := r.data.DB.WithContext(ctx).Where("app_key = ?", appKey).First(&po).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.MerchantFromPO(&po), nil
}

func (r *repo) List(ctx context.Context, q bizmerchant.Query, page, pageSize int) ([]*bizmerchant.Merchant, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.MerchantPO{})
	// 按字段独立全模糊匹配；转义用户输入中的 LIKE 通配符，防止 %/_ 改变匹配语义（通配符注入）
	if q.Name != "" {
		tx = tx.Where("name LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Name)+"%")
	}
	if q.Code != "" {
		tx = tx.Where("code LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Code)+"%")
	}
	if q.AppKey != "" {
		tx = tx.Where("app_key LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.AppKey)+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.MerchantPO
	// 列表查询不带密钥哈希
	if err := tx.Omit("app_secret_hash").Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*bizmerchant.Merchant, 0, len(pos))
	for i := range pos {
		out = append(out, model.MerchantFromPO(&pos[i]))
	}
	return out, total, nil
}

// ---- API 调用日志仓储（写异步、读同步） ----

// 写入队列容量：超出即丢弃并告警（日志属于尽力而为的旁路数据，不反压业务主流程）
const queueSize = 256

// APILogRepo 调用日志仓储
type APILogRepo struct {
	data  *data.Data
	queue chan *model.MerchantAPILogPO
	done  chan struct{} // 通知保留期清理循环退出
	wg    sync.WaitGroup
}

// NewAPILogRepo 创建调用日志仓储：启动异步写入 worker；
// retentionDays > 0 时启动每日保留期清理循环（复用 conf.Log.RetentionDays）。
// cleanup 冲刷剩余队列并停止后台协程（关停时先于数据库连接关闭执行）。
func NewAPILogRepo(d *data.Data, c *conf.Bootstrap) (bizmerchant.APILogRepo, func(), error) {
	r := &APILogRepo{
		data:  d,
		queue: make(chan *model.MerchantAPILogPO, queueSize),
		done:  make(chan struct{}),
	}
	r.wg.Add(1)
	go r.writeWorker()
	if c.Log.RetentionDays > 0 {
		r.wg.Add(1)
		go r.retentionLoop(c.Log.RetentionDays)
	}
	cleanup := func() {
		close(r.done)
		close(r.queue) // worker 消费完剩余条目后退出
		r.wg.Wait()
	}
	return r, cleanup, nil
}

// writeWorker 异步写入协程：逐条落库（写频低，无需攒批），失败告警不重试
func (r *APILogRepo) writeWorker() {
	defer r.wg.Done()
	ctx := context.Background()
	for po := range r.queue {
		if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
			logger.Warn("merchant api log write failed", zap.Error(err))
		}
	}
}

func (r *APILogRepo) Record(l *bizmerchant.APILog) {
	select {
	case r.queue <- model.MerchantAPILogToPO(l):
	default:
		logger.Warn("merchant api log queue full, dropped", zap.String("app_key", l.AppKey))
	}
}

func (r *APILogRepo) List(ctx context.Context, q bizmerchant.APILogQuery, page, pageSize int) ([]*bizmerchant.APILog, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.MerchantAPILogPO{})
	if q.AppKey != "" {
		tx = tx.Where("app_key = ?", q.AppKey)
	}
	if q.Path != "" {
		tx = tx.Where("path LIKE ? ESCAPE '/'", security.EscapeLike(q.Path)+"%")
	}
	if q.StatusCode != nil {
		tx = tx.Where("status_code = ?", *q.StatusCode)
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
	var pos []model.MerchantAPILogPO
	if err := tx.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*bizmerchant.APILog, 0, len(pos))
	for i := range pos {
		out = append(out, model.MerchantAPILogFromPO(&pos[i]))
	}
	return out, total, nil
}

// retentionLoop 保留期清理循环：启动先跑一次，此后每日一次（单机版定时，无需分布式锁）
func (r *APILogRepo) retentionLoop(days int) {
	defer r.wg.Done()
	r.cleanupExpired(days)
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-r.done:
			return
		case <-t.C:
			r.cleanupExpired(days)
		}
	}
}

// cleanupExpired 物理删除保留期外的调用日志
func (r *APILogRepo) cleanupExpired(days int) {
	ctx := context.Background()
	cutoff := time.Now().AddDate(0, 0, -days)
	res := r.data.DB.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&model.MerchantAPILogPO{})
	if res.Error != nil {
		logger.Warn("merchant api log retention cleanup failed", zap.Error(res.Error))
		return
	}
	if res.RowsAffected > 0 {
		logger.Info("merchant api log retention cleanup", zap.Int64("deleted", res.RowsAffected))
	}
}
