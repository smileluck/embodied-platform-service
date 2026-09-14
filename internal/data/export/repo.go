// Package export 导出记录仓储 GORM 实现与异步导出 worker：
// 任务经内存队列串行执行（单 goroutine，避免并发导出击穿数据库），
// 分批拉数写临时 CSV 后存入当前存储后端，附保留期每日自动清理。
package export

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"
	"time"

	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// fetchBatchSize worker 分批拉数的批大小（offset/limit 语义，逐批写盘控制内存）
const fetchBatchSize = 1000

// repo 导出记录仓储（同步 CRUD；任务执行在 Worker）
type repo struct {
	data *data.Data
}

// NewRepo 创建导出记录仓储
func NewRepo(d *data.Data) bizexport.Repo { return &repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return bizexport.ErrNotFound
	}
	return err
}

func (r *repo) Create(ctx context.Context, rec *bizexport.ExportRecord) error {
	po := model.ExportRecordToPO(rec)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	// 回写自增主键与时间戳，供应用层返回给前端
	rec.ID, rec.CreatedAt = po.ID, po.CreatedAt
	return nil
}

func (r *repo) Update(ctx context.Context, rec *bizexport.ExportRecord) error {
	return r.data.DB.WithContext(ctx).Save(model.ExportRecordToPO(rec)).Error
}

func (r *repo) ListByUser(ctx context.Context, userID uint, page, pageSize int) ([]*bizexport.ExportRecord, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.ExportRecordPO{}).Where("user_id = ?", userID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.ExportRecordPO
	if err := tx.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*bizexport.ExportRecord, 0, len(pos))
	for i := range pos {
		out = append(out, model.ExportRecordFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *repo) RecentByUser(ctx context.Context, userID uint, limit int) ([]*bizexport.ExportRecord, error) {
	var pos []model.ExportRecordPO
	if err := r.data.DB.WithContext(ctx).
		Where("user_id = ?", userID).Order("id DESC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*bizexport.ExportRecord, 0, len(pos))
	for i := range pos {
		out = append(out, model.ExportRecordFromPO(&pos[i]))
	}
	return out, nil
}

func (r *repo) FindByID(ctx context.Context, id uint) (*bizexport.ExportRecord, error) {
	var po model.ExportRecordPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.ExportRecordFromPO(&po), nil
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.ExportRecordPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return bizexport.ErrNotFound
	}
	return nil
}

func (r *repo) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.data.DB.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&model.ExportRecordPO{})
	return res.RowsAffected, res.Error
}

// Worker 异步导出执行器：buffered channel + 单 goroutine 串行消费 + WaitGroup 优雅退出，
// 附保留期每日清理循环（模式与 internal/data/log 的异步写入 worker 一致）
type Worker struct {
	data     *data.Data
	cfg      *conf.Bootstrap
	registry *bizexport.Registry
	storage  *bizfile.StorageManager
	repo     bizexport.Repo
	queue    chan uint
	done     chan struct{} // 通知保留期清理循环退出
	wg       sync.WaitGroup
}

// NewWorker 创建导出 worker：启动前把上次运行残留的 pending/running 任务置 failed（进程已死，任务不可能继续），
// 随后启动消费协程；retentionDays > 0 时启动每日保留期清理循环。
// cleanup 停止后台协程并等待在途任务写完（关停时先于数据库连接关闭执行）。
func NewWorker(d *data.Data, c *conf.Bootstrap, registry *bizexport.Registry, storageMgr *bizfile.StorageManager, repo bizexport.Repo) (*Worker, func(), error) {
	queueSize := c.Export.QueueSize
	if queueSize <= 0 {
		queueSize = 64
	}
	w := &Worker{
		data: d, cfg: c, registry: registry, storage: storageMgr, repo: repo,
		queue: make(chan uint, queueSize),
		done:  make(chan struct{}),
	}
	// 启动恢复：残留 pending/running 的任务随进程退出已中断，统一置 failed
	if err := d.DB.Model(&model.ExportRecordPO{}).
		Where("status IN ?", []string{bizexport.StatusPending, bizexport.StatusRunning}).
		Updates(map[string]interface{}{
			"status":      bizexport.StatusFailed,
			"error":       "服务重启，任务中断",
			"finished_at": time.Now(),
		}).Error; err != nil {
		return nil, nil, err
	}
	w.wg.Add(1)
	go w.consumeLoop()
	if c.Export.RetentionDays > 0 {
		w.wg.Add(1)
		go w.retentionLoop(c.Export.RetentionDays)
	}
	cleanup := func() {
		close(w.done)
		close(w.queue) // worker 消费完剩余任务后退出
		w.wg.Wait()
	}
	return w, cleanup, nil
}

// Enqueue 任务入队（实现 bizexport.Enqueuer）；队列满返回 ErrQueueFull，由提交方回滚记录
func (w *Worker) Enqueue(id uint) error {
	select {
	case w.queue <- id:
		return nil
	default:
		return bizexport.ErrQueueFull
	}
}

// consumeLoop 任务消费协程：串行执行，单任务失败不影响后续任务
func (w *Worker) consumeLoop() {
	defer w.wg.Done()
	for id := range w.queue {
		w.process(id)
	}
}

// process 执行单个导出任务：状态推进 running → done/failed，任何失败落 error 不抛出
func (w *Worker) process(id uint) {
	ctx := context.Background()
	rec, err := w.repo.FindByID(ctx, id)
	if err != nil {
		logger.Warn("export record load failed", zap.Uint("id", id), zap.Error(err))
		return
	}
	if rec.Status != bizexport.StatusPending {
		return // 重复入队或已被启动恢复标记，跳过
	}
	rec.Status = bizexport.StatusRunning
	if err := w.repo.Update(ctx, rec); err != nil {
		logger.Warn("export mark running failed", zap.Uint("id", id), zap.Error(err))
		return
	}
	if err := w.run(ctx, rec); err != nil {
		logger.Warn("export task failed", zap.Uint("id", id), zap.String("biz", rec.Biz), zap.Error(err))
		rec.Status = bizexport.StatusFailed
		rec.Error = truncate(err.Error(), 512)
		now := time.Now()
		rec.FinishedAt = &now
		if uerr := w.repo.Update(ctx, rec); uerr != nil {
			logger.Warn("export mark failed failed", zap.Uint("id", id), zap.Error(uerr))
		}
	}
}

// run 拉数写 CSV 并入库产物：临时文件 → UTF-8 BOM + 表头 → 分批 Fetch 写行
// （触及大小/行数上限即截断并标记 Truncated）→ 存入当前存储后端 → 更新记录
func (w *Worker) run(ctx context.Context, rec *bizexport.ExportRecord) error {
	exp, ok := w.registry.Get(rec.Biz)
	if !ok {
		return bizexport.ErrUnsupportedBiz
	}
	var params url.Values
	if err := json.Unmarshal([]byte(rec.Params), &params); err != nil {
		return fmt.Errorf("解析查询条件失败: %w", err)
	}
	tmp, err := os.CreateTemp("", "export-*.csv")
	if err != nil {
		return err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	maxBytes := w.cfg.Export.MaxSizeMB << 20
	if maxBytes <= 0 {
		maxBytes = 50 << 20
	}
	maxRows := w.cfg.Export.MaxRows
	if maxRows <= 0 {
		maxRows = 100000
	}

	// 计数 writer：csv.Writer 的每次落盘都计入产物大小（含 BOM），超限即截断
	cw := &countingWriter{w: tmp}
	if _, err := cw.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil { // UTF-8 BOM，Excel 直开不乱码
		return err
	}
	wtr := csv.NewWriter(cw)
	cols := exp.Columns()
	header := make([]string, 0, len(cols))
	for _, c := range cols {
		header = append(header, c.Title)
	}
	if err := wtr.Write(header); err != nil {
		return err
	}
	wtr.Flush()
	if err := wtr.Error(); err != nil {
		return err
	}

	rows := 0
	truncated := false
	offset := 0
	for {
		batch, total, err := exp.Fetch(ctx, params, offset, fetchBatchSize)
		if err != nil {
			return err
		}
		for _, row := range batch {
			if rows >= maxRows || cw.n >= maxBytes {
				truncated = true
				break
			}
			if err := wtr.Write(row); err != nil {
				return err
			}
			wtr.Flush() // 逐行冲刷以便 cw.n 反映真实大小（CSV 转义后的体积）
			if err := wtr.Error(); err != nil {
				return err
			}
			rows++
		}
		if truncated || len(batch) < fetchBatchSize || int64(offset+len(batch)) >= total {
			break
		}
		offset += fetchBatchSize
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	driver := w.storage.Current().Driver()
	key := genObjectKey()
	if err := w.storage.Current().Put(ctx, key, tmp, cw.n, "text/csv"); err != nil {
		return err
	}

	rec.Status = bizexport.StatusDone
	rec.Driver = driver
	rec.ObjectKey = key
	rec.Size = cw.n
	rec.Rows = rows
	rec.Truncated = truncated
	now := time.Now()
	rec.FinishedAt = &now
	if err := w.repo.Update(ctx, rec); err != nil {
		// 落库失败则产物成为孤儿对象，尽力清理
		if derr := w.storage.Current().Delete(context.Background(), key); derr != nil {
			logger.Warn("export result save failed, orphan object cleanup failed",
				zap.String("key", key), zap.Error(derr))
		}
		return err
	}
	logger.Info("export task done",
		zap.Uint("id", rec.ID), zap.String("biz", rec.Biz),
		zap.Int("rows", rows), zap.Int64("size", cw.n), zap.Bool("truncated", truncated))
	return nil
}

// retentionLoop 保留期清理循环：启动先跑一次，此后每日一次（单机版定时，无需分布式锁）
func (w *Worker) retentionLoop(days int) {
	defer w.wg.Done()
	w.cleanupExpired(days)
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-t.C:
			w.cleanupExpired(days)
		}
	}
}

// cleanupExpired 清理保留期外的导出任务：先尽力删除存储产物，再物理删除记录
func (w *Worker) cleanupExpired(days int) {
	ctx := context.Background()
	cutoff := time.Now().AddDate(0, 0, -days)
	var pos []model.ExportRecordPO
	if err := w.data.DB.WithContext(ctx).Select("id", "driver", "object_key").
		Where("created_at < ?", cutoff).Find(&pos).Error; err != nil {
		logger.Warn("export retention cleanup query failed", zap.Error(err))
		return
	}
	for i := range pos {
		if pos[i].ObjectKey == "" {
			continue
		}
		if backend, err := w.storage.For(pos[i].Driver); err == nil {
			if derr := backend.Delete(ctx, pos[i].ObjectKey); derr != nil {
				logger.Warn("export retention object delete failed",
					zap.String("key", pos[i].ObjectKey), zap.Error(derr))
			}
		}
	}
	n, err := w.repo.DeleteBefore(ctx, cutoff)
	if err != nil {
		logger.Warn("export retention cleanup failed", zap.Error(err))
		return
	}
	if n > 0 {
		logger.Info("export retention cleanup", zap.Int64("records_deleted", n))
	}
}

// countingWriter 统计写入字节数（产物大小与截断判定共用同一真实口径）
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// genObjectKey 服务端生成产物对象 key：exports/yyyy/mm/<24hex>.csv，杜绝用户控制存储路径
func genObjectKey() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "exports/" + time.Now().Format("2006/01") + "/" + hex.EncodeToString(b) + ".csv"
}

// truncate 按字节长度截断（失败原因入库用）
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
