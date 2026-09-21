// Package notice 通知公告仓储 GORM 实现
package notice

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/smilex/smilex-admin-gin/internal/biz/notice"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
)

type repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) notice.Repo { return &repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notice.ErrNotFound
	}
	return err
}

func (r *repo) Create(ctx context.Context, n *notice.Notice) error {
	po := model.NoticeToPO(n)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	n.ID, n.CreatedAt, n.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) Update(ctx context.Context, n *notice.Notice) error {
	return r.data.DB.WithContext(ctx).Model(&model.NoticePO{}).Where("id = ?", n.ID).
		Updates(map[string]interface{}{
			"title": n.Title, "content": n.Content, "level": string(n.Level),
			"publish_at": n.PublishAt, "expire_at": n.ExpireAt,
		}).Error
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.NoticePO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notice.ErrNotFound
	}
	return nil
}

func (r *repo) Find(ctx context.Context, id uint) (*notice.Notice, error) {
	var po model.NoticePO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.NoticeFromPO(&po), nil
}

func (r *repo) List(ctx context.Context, q notice.Query, page, pageSize int) ([]*notice.Notice, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.NoticePO{})
	if q.Title != "" {
		tx = tx.Where("title LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Title)+"%")
	}
	if q.Level != "" {
		tx = tx.Where("level = ?", q.Level)
	}
	now := time.Now()
	switch q.Status {
	case "active":
		tx = tx.Where("publish_at <= ? AND (expire_at IS NULL OR expire_at > ?)", now, now)
	case "expired":
		tx = tx.Where("expire_at IS NOT NULL AND expire_at <= ?", now)
	case "pending":
		tx = tx.Where("publish_at > ?", now)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.NoticePO
	if err := tx.Offset((page - 1) * pageSize).Limit(pageSize).
		Order("publish_at DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*notice.Notice, 0, len(pos))
	for i := range pos {
		out = append(out, model.NoticeFromPO(&pos[i]))
	}
	return out, total, nil
}

// ListActive 生效中公告 + 本人已读标记（LEFT JOIN notice_reads）
func (r *repo) ListActive(ctx context.Context, userID uint) ([]*notice.Notice, error) {
	now := time.Now()
	var rows []struct {
		model.NoticePO
		ReadID *uint `gorm:"column:read_id"`
	}
	err := r.data.DB.WithContext(ctx).
		Select("notices.*, notice_reads.id AS read_id").
		Joins("LEFT JOIN notice_reads ON notice_reads.notice_id = notices.id AND notice_reads.user_id = ?", userID).
		Where("notices.publish_at <= ? AND (notices.expire_at IS NULL OR notices.expire_at > ?)", now, now).
		Order("notices.publish_at DESC").Limit(50).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*notice.Notice, 0, len(rows))
	for i := range rows {
		n := model.NoticeFromPO(&rows[i].NoticePO)
		n.HasRead = rows[i].ReadID != nil
		out = append(out, n)
	}
	return out, nil
}

func (r *repo) CountUnread(ctx context.Context, userID uint) (int64, error) {
	now := time.Now()
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.NoticePO{}).
		Where("publish_at <= ? AND (expire_at IS NULL OR expire_at > ?)", now, now).
		Where("id NOT IN (SELECT notice_id FROM notice_reads WHERE user_id = ?)", userID).
		Count(&n).Error
	return n, err
}

func (r *repo) MarkRead(ctx context.Context, userID, noticeID uint) error {
	// 幂等：唯一键冲突视为成功
	return r.data.DB.WithContext(ctx).
		Where("user_id = ? AND notice_id = ?", userID, noticeID).
		FirstOrCreate(&model.NoticeReadPO{UserID: userID, NoticeID: noticeID}).Error
}
