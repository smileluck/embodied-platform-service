// Package job 定时任务仓储 GORM 实现
package job

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/smilex/smilex-admin-gin/internal/biz/job"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
)

type repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) job.Repo { return &repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return job.ErrNotFound
	}
	return err
}

func (r *repo) Create(ctx context.Context, j *job.Job) error {
	po := model.JobToPO(j)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	j.ID, j.CreatedAt, j.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) Update(ctx context.Context, j *job.Job) error {
	return r.data.DB.WithContext(ctx).Model(&model.JobPO{}).Where("id = ?", j.ID).
		Updates(map[string]interface{}{
			"name": j.Name, "cron": j.Cron, "handler_key": j.HandlerKey,
			"params": j.Params, "remark": j.Remark, "status": int(j.Status), "last_run_at": j.LastRunAt,
		}).Error
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.JobPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return job.ErrNotFound
	}
	return nil
}

func (r *repo) Find(ctx context.Context, id uint) (*job.Job, error) {
	var po model.JobPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.JobFromPO(&po), nil
}

func (r *repo) List(ctx context.Context, q job.Query, page, pageSize int) ([]*job.Job, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.JobPO{})
	if q.Name != "" {
		tx = tx.Where("name LIKE ? ESCAPE '/'", security.EscapeLike(q.Name)+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.JobPO
	if pageSize > 0 {
		tx = tx.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	if err := tx.Order("id ASC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*job.Job, 0, len(pos))
	for i := range pos {
		out = append(out, model.JobFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *repo) ListEnabled(ctx context.Context) ([]*job.Job, error) {
	var pos []model.JobPO
	if err := r.data.DB.WithContext(ctx).Where("status = 1").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*job.Job, 0, len(pos))
	for i := range pos {
		out = append(out, model.JobFromPO(&pos[i]))
	}
	return out, nil
}

func (r *repo) AppendLog(ctx context.Context, l *job.JobLog) error {
	po := model.JobLogToPO(l)
	po.StartedAt = time.Now()
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	l.ID = po.ID
	return nil
}

func (r *repo) ListLogs(ctx context.Context, jobID uint, page, pageSize int) ([]*job.JobLog, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.JobLogPO{}).Where("job_id = ?", jobID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.JobLogPO
	if err := tx.Offset((page - 1) * pageSize).Limit(pageSize).
		Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*job.JobLog, 0, len(pos))
	for i := range pos {
		out = append(out, model.JobLogFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *repo) CleanupLogsBefore(ctx context.Context, before time.Time) error {
	return r.data.DB.WithContext(ctx).Where("started_at < ?", before).
		Delete(&model.JobLogPO{}).Error
}
