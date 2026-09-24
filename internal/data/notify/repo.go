// Package notify 告警通知仓储 GORM 实现
package notify

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/smilex/smilex-admin-gin/internal/biz/notify"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
)

type Repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) notify.Repo { return &Repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notify.ErrChannelNotFound
	}
	return err
}

// ---- 渠道 ----

func (r *Repo) CreateChannel(ctx context.Context, ch *notify.Channel) error {
	po := model.NotifyChannelToPO(ch)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	ch.ID, ch.CreatedAt, ch.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *Repo) UpdateChannel(ctx context.Context, ch *notify.Channel) error {
	return r.data.DB.WithContext(ctx).Model(&model.NotifyChannelPO{}).Where("id = ?", ch.ID).
		Updates(map[string]interface{}{
			"name": ch.Name, "type": string(ch.Type), "status": ch.Status,
			"smtp_host": ch.SMTPHost, "smtp_port": ch.SMTPPort, "smtp_user": ch.SMTPUser,
			"smtp_pass_enc": ch.SMTPPassEnc, "smtp_pass_mask": ch.SMTPPassMask, "smtp_from": ch.SMTPFrom,
			"recipients":  model.NotifyChannelToPO(ch).Recipients,
			"webhook_url": ch.WebhookURL, "webhook_enc": ch.WebhookEnc, "webhook_mask": ch.WebhookMask,
		}).Error
}

func (r *Repo) DeleteChannel(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.NotifyChannelPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notify.ErrChannelNotFound
	}
	return nil
}

func (r *Repo) FindChannel(ctx context.Context, id uint) (*notify.Channel, error) {
	var po model.NotifyChannelPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.NotifyChannelFromPO(&po), nil
}

func (r *Repo) ListChannels(ctx context.Context, q notify.ChannelQuery) ([]*notify.Channel, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.NotifyChannelPO{})
	if q.Name != "" {
		// 转义用户输入中的 LIKE 通配符，防止 %/_ 改变匹配语义（通配符注入）
		tx = tx.Where("name LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Name)+"%")
	}
	if q.Type != "" {
		tx = tx.Where("type = ?", q.Type)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var pos []model.NotifyChannelPO
	if err := tx.Order("id ASC").Limit(500).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*notify.Channel, 0, len(pos))
	for i := range pos {
		out = append(out, model.NotifyChannelFromPO(&pos[i]))
	}
	return out, nil
}

// ---- 规则 ----

func (r *Repo) CreateRule(ctx context.Context, rule *notify.Rule) error {
	po := model.NotifyRuleToPO(rule)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	rule.ID, rule.CreatedAt, rule.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *Repo) UpdateRule(ctx context.Context, rule *notify.Rule) error {
	return r.data.DB.WithContext(ctx).Model(&model.NotifyRulePO{}).Where("id = ?", rule.ID).
		Updates(map[string]interface{}{
			"name": rule.Name, "source": string(rule.Source), "metric": string(rule.Metric),
			"threshold": rule.Threshold, "channel_ids": model.NotifyRuleToPO(rule).ChannelIDs,
			"cooldown_seconds": rule.CooldownSeconds, "status": rule.Status, "remark": rule.Remark,
		}).Error
}

func (r *Repo) DeleteRule(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.NotifyRulePO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notify.ErrRuleNotFound
	}
	return nil
}

func (r *Repo) FindRule(ctx context.Context, id uint) (*notify.Rule, error) {
	var po model.NotifyRulePO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, notify.ErrRuleNotFound
		}
		return nil, err
	}
	return model.NotifyRuleFromPO(&po), nil
}

func (r *Repo) ListRules(ctx context.Context, q notify.RuleQuery) ([]*notify.Rule, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.NotifyRulePO{})
	if q.Name != "" {
		// 转义用户输入中的 LIKE 通配符，防止 %/_ 改变匹配语义（通配符注入）
		tx = tx.Where("name LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Name)+"%")
	}
	if q.Source != "" {
		tx = tx.Where("source = ?", q.Source)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var pos []model.NotifyRulePO
	if err := tx.Order("id ASC").Limit(500).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*notify.Rule, 0, len(pos))
	for i := range pos {
		out = append(out, model.NotifyRuleFromPO(&pos[i]))
	}
	return out, nil
}

// ---- 分发器 ----

func (r *Repo) ListEnabledRules(ctx context.Context) ([]*notify.Rule, error) {
	var pos []model.NotifyRulePO
	if err := r.data.DB.WithContext(ctx).Where("status = ?", notify.StatusEnabled).Order("id ASC").Limit(500).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*notify.Rule, 0, len(pos))
	for i := range pos {
		out = append(out, model.NotifyRuleFromPO(&pos[i]))
	}
	return out, nil
}

func (r *Repo) FindEnabledChannels(ctx context.Context, ids []uint) ([]*notify.Channel, error) {
	if len(ids) == 0 {
		return []*notify.Channel{}, nil
	}
	var pos []model.NotifyChannelPO
	if err := r.data.DB.WithContext(ctx).
		Where("id IN ? AND status = ?", ids, notify.StatusEnabled).
		Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*notify.Channel, 0, len(pos))
	for i := range pos {
		out = append(out, model.NotifyChannelFromPO(&pos[i]))
	}
	return out, nil
}

func (r *Repo) TouchRule(ctx context.Context, id uint, at time.Time) error {
	return r.data.DB.WithContext(ctx).Model(&model.NotifyRulePO{}).Where("id = ?", id).
		Update("last_triggered_at", at).Error
}

// ---- 发送记录 ----

func (r *Repo) AppendRecord(ctx context.Context, rec *notify.Record) error {
	po := model.NotifyRecordToPO(rec)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	rec.ID, rec.CreatedAt = po.ID, po.CreatedAt
	return nil
}

func (r *Repo) ListRecords(ctx context.Context, q notify.RecordQuery, page, pageSize int) ([]*notify.Record, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.NotifyRecordPO{})
	if q.ChannelID != 0 {
		tx = tx.Where("channel_id = ?", q.ChannelID)
	}
	if q.Source != "" {
		tx = tx.Where("source = ?", q.Source)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.NotifyRecordPO
	if pageSize > 0 { // pageSize<=0 表示全量（不分页）
		tx = tx.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	if err := tx.Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*notify.Record, 0, len(pos))
	for i := range pos {
		out = append(out, model.NotifyRecordFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *Repo) ClearRecords(ctx context.Context) error {
	return r.data.DB.WithContext(ctx).
		Where("status IN ?", []string{notify.RecordSent, notify.RecordFailed}).
		Delete(&model.NotifyRecordPO{}).Error
}

func (r *Repo) DeleteRecordsBefore(ctx context.Context, before time.Time) error {
	return r.data.DB.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&model.NotifyRecordPO{}).Error
}
