// Package admission 准入投影仓储 GORM 实现
package admission

import (
	"context"
	"errors"

	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"gorm.io/gorm"
)

type repo struct {
	data *data.Data
}

// NewRepo 创建准入投影仓储
func NewRepo(d *data.Data) bizadmission.Repo { return &repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return bizadmission.ErrNotFound
	}
	return err
}

// loadRoles 加载投影的角色绑定
func loadRoles(ctx context.Context, tx *gorm.DB, p *bizadmission.Projection) error {
	var rels []model.PlatformUserRolePO
	if err := tx.WithContext(ctx).
		Where("platform_user_id = ?", p.PlatformUserID).
		Order("role_id").Find(&rels).Error; err != nil {
		return err
	}
	p.RoleIDs = make([]uint, 0, len(rels))
	for _, r := range rels {
		p.RoleIDs = append(p.RoleIDs, r.RoleID)
	}
	return nil
}

func (r *repo) Create(ctx context.Context, p *bizadmission.Projection) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po := model.PlatformUserToPO(p)
		if err := tx.Create(po).Error; err != nil {
			return err
		}
		p.ID, p.CreatedAt, p.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
		return replaceRoles(ctx, tx, p.PlatformUserID, p.RoleIDs)
	})
}

func (r *repo) Update(ctx context.Context, p *bizadmission.Projection) error {
	return r.data.DB.WithContext(ctx).Model(&model.PlatformUserPO{}).
		Where("id = ?", p.ID).
		Updates(map[string]interface{}{
			"username": p.Username, "nickname": p.Nickname, "email": p.Email, "enabled": p.Enabled,
		}).Error
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var po model.PlatformUserPO
		if err := tx.First(&po, id).Error; err != nil {
			return mapErr(err)
		}
		if err := tx.Delete(&model.PlatformUserPO{}, id).Error; err != nil {
			return err
		}
		// 投影删除为物理删除（重新准入=重建投影）
		return tx.Where("platform_user_id = ?", po.PlatformUserID).
			Delete(&model.PlatformUserRolePO{}).Error
	})
}

func (r *repo) FindByID(ctx context.Context, id uint) (*bizadmission.Projection, error) {
	var po model.PlatformUserPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	p := model.PlatformUserFromPO(&po)
	if err := loadRoles(ctx, r.data.DB, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *repo) FindByPlatformUserID(ctx context.Context, platformUserID uint) (*bizadmission.Projection, error) {
	var po model.PlatformUserPO
	err := r.data.DB.WithContext(ctx).Where("platform_user_id = ?", platformUserID).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 未建投影：认证链路据此判 403，非错误
	}
	if err != nil {
		return nil, err
	}
	p := model.PlatformUserFromPO(&po)
	if err := loadRoles(ctx, r.data.DB, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *repo) FindByPlatformUserIDs(ctx context.Context, platformUserIDs []uint) (map[uint]*bizadmission.Projection, error) {
	out := make(map[uint]*bizadmission.Projection, len(platformUserIDs))
	if len(platformUserIDs) == 0 {
		return out, nil
	}
	var pos []model.PlatformUserPO
	if err := r.data.DB.WithContext(ctx).
		Where("platform_user_id IN ?", platformUserIDs).Find(&pos).Error; err != nil {
		return nil, err
	}
	for i := range pos {
		p := model.PlatformUserFromPO(&pos[i])
		if err := loadRoles(ctx, r.data.DB, p); err != nil {
			return nil, err
		}
		out[p.PlatformUserID] = p
	}
	return out, nil
}

func (r *repo) List(ctx context.Context, q bizadmission.Query, page, pageSize int) ([]*bizadmission.Projection, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.PlatformUserPO{})
	if q.Username != "" {
		tx = tx.Where("username LIKE ?", q.Username+"%")
	}
	if q.Status != nil {
		tx = tx.Where("enabled = ?", *q.Status == 1)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.PlatformUserPO
	if pageSize > 0 {
		tx = tx.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	if err := tx.Order("id").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*bizadmission.Projection, 0, len(pos))
	for i := range pos {
		p := model.PlatformUserFromPO(&pos[i])
		if err := loadRoles(ctx, r.data.DB, p); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, nil
}

func (r *repo) SetRoles(ctx context.Context, id uint, roleIDs []uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var po model.PlatformUserPO
		if err := tx.First(&po, id).Error; err != nil {
			return mapErr(err)
		}
		return replaceRoles(ctx, tx, po.PlatformUserID, roleIDs)
	})
}

// replaceRoles 全量替换角色绑定（物理删除旧关联）
func replaceRoles(ctx context.Context, tx *gorm.DB, platformUserID uint, roleIDs []uint) error {
	if err := tx.Where("platform_user_id = ?", platformUserID).
		Delete(&model.PlatformUserRolePO{}).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rels := make([]model.PlatformUserRolePO, 0, len(roleIDs))
	for _, rid := range roleIDs {
		rels = append(rels, model.PlatformUserRolePO{PlatformUserID: platformUserID, RoleID: rid})
	}
	return tx.Create(&rels).Error
}
