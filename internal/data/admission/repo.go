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

// ListPlatformUserIDs 全部投影的平台用户 ID 集（孤儿投影对账回收用）
func (r *repo) ListPlatformUserIDs(ctx context.Context) ([]uint, error) {
	var ids []uint
	if err := r.data.DB.WithContext(ctx).Model(&model.PlatformUserPO{}).
		Order("id").Pluck("platform_user_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *repo) SetRoles(ctx context.Context, id uint, roleIDs []uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var po model.PlatformUserPO
		if err := tx.First(&po, id).Error; err != nil {
			return mapErr(err)
		}
		// 全量替换前保留现持有的内置锁定角色（超管=配置引导、商户管理员=平台标记驱动）：
		// 手工分配不得增删锁定绑定，请求里显式携带的锁定 ID 一并忽略
		var lockedRels []model.PlatformUserRolePO
		if err := tx.Where("platform_user_id = ? AND role_id IN ?", po.PlatformUserID, bizadmission.LockedRoleIDs).
			Find(&lockedRels).Error; err != nil {
			return err
		}
		kept := make([]uint, 0, len(lockedRels))
		for _, rel := range lockedRels {
			kept = append(kept, rel.RoleID)
		}
		merged := make([]uint, 0, len(roleIDs)+len(kept))
		for _, rid := range roleIDs {
			if !bizadmission.IsLockedRole(rid) {
				merged = append(merged, rid)
			}
		}
		merged = append(merged, kept...)
		return replaceRoles(ctx, tx, po.PlatformUserID, merged)
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

// GrantRole 追加单个角色绑定（幂等；唯一索引兜底并发）
func (r *repo) GrantRole(ctx context.Context, platformUserID, roleID uint) error {
	po := model.PlatformUserRolePO{PlatformUserID: platformUserID, RoleID: roleID}
	return r.data.DB.WithContext(ctx).
		Where("platform_user_id = ? AND role_id = ?", platformUserID, roleID).
		FirstOrCreate(&po).Error
}

// RevokeRole 移除单个角色绑定（幂等；未持有视为已达成目标状态）
func (r *repo) RevokeRole(ctx context.Context, platformUserID, roleID uint) error {
	return r.data.DB.WithContext(ctx).
		Where("platform_user_id = ? AND role_id = ?", platformUserID, roleID).
		Delete(&model.PlatformUserRolePO{}).Error
}

// RoleHolderUserIDs 持有指定角色的平台用户 ID 集
func (r *repo) RoleHolderUserIDs(ctx context.Context, roleID uint) ([]uint, error) {
	var ids []uint
	if err := r.data.DB.WithContext(ctx).Model(&model.PlatformUserRolePO{}).
		Where("role_id = ?", roleID).Distinct().Pluck("platform_user_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
