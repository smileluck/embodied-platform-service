// Package sysconfig 系统参数仓储 GORM 实现
package sysconfig

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/internal/biz/sysconfig"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

type repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) sysconfig.Repo { return &repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sysconfig.ErrNotFound
	}
	return err
}

func (r *repo) Create(ctx context.Context, c *sysconfig.Config) error {
	po := model.SysConfigToPO(c)
	return r.data.DB.WithContext(ctx).Create(po).Error
}

func (r *repo) Update(ctx context.Context, c *sysconfig.Config) error {
	return r.data.DB.WithContext(ctx).Model(&model.SysConfigPO{}).Where("`key` = ?", c.Key).
		Updates(map[string]interface{}{"value": c.Value, "description": c.Description}).Error
}

func (r *repo) Delete(ctx context.Context, key string) error {
	res := r.data.DB.WithContext(ctx).Where("`key` = ?", key).Delete(&model.SysConfigPO{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return sysconfig.ErrNotFound
	}
	return nil
}

func (r *repo) Find(ctx context.Context, key string) (*sysconfig.Config, error) {
	var po model.SysConfigPO
	if err := r.data.DB.WithContext(ctx).Where("`key` = ?", key).First(&po).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.SysConfigFromPO(&po), nil
}

func (r *repo) List(ctx context.Context, keyword string) ([]*sysconfig.Config, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.SysConfigPO{})
	if keyword != "" {
		tx = tx.Where("`key` LIKE ? ESCAPE '/' OR description LIKE ? ESCAPE '/'",
			"%"+security.EscapeLike(keyword)+"%", "%"+security.EscapeLike(keyword)+"%")
	}
	var pos []model.SysConfigPO
	if err := tx.Order("`key` ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*sysconfig.Config, 0, len(pos))
	for i := range pos {
		out = append(out, model.SysConfigFromPO(&pos[i]))
	}
	return out, nil
}
