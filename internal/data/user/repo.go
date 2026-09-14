// Package user 用户仓储 GORM 实现
package user

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/internal/biz/user"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

type repo struct {
	data *data.Data
}

// NewRepo 创建用户仓储
func NewRepo(d *data.Data) user.Repo { return &repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return user.ErrUserNotFound
	}
	return err
}

func (r *repo) loadRoles(ctx context.Context, u *user.User) error {
	var rs []model.UserRolePO
	if err := r.data.DB.WithContext(ctx).Where("user_id = ?", u.ID).Find(&rs).Error; err != nil {
		return err
	}
	for _, x := range rs {
		u.RoleIDs = append(u.RoleIDs, x.RoleID)
	}
	return nil
}

func (r *repo) Create(ctx context.Context, u *user.User) error {
	po := model.UserToPO(u)
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(po).Error; err != nil {
			return err
		}
		u.ID = po.ID
		u.CreatedAt, u.UpdatedAt = po.CreatedAt, po.UpdatedAt
		return r.replaceRoles(tx, u.ID, u.RoleIDs)
	})
}

func (r *repo) replaceRoles(tx *gorm.DB, userID uint, roleIDs []uint) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserRolePO{}).Error; err != nil {
		return err
	}
	for _, rid := range roleIDs {
		if err := tx.Create(&model.UserRolePO{UserID: userID, RoleID: rid}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *repo) Update(ctx context.Context, u *user.User) error {
	po := model.UserToPO(u)
	// 只更新基础资料字段；密码一律走 UpdatePassword，避免无哈希查询后被意外清空
	return r.data.DB.WithContext(ctx).Model(&model.UserPO{}).Where("id = ?", u.ID).
		Updates(map[string]interface{}{
			"nickname": po.Nickname, "phone": po.Phone, "email": po.Email, "status": po.Status,
		}).Error
}

func (r *repo) UpdatePassword(ctx context.Context, id uint, passwordHash string) error {
	return r.data.DB.WithContext(ctx).Model(&model.UserPO{}).Where("id = ?", id).
		Updates(map[string]interface{}{"password": passwordHash}).Error
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.UserPO{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return user.ErrUserNotFound
		}
		return tx.Where("user_id = ?", id).Delete(&model.UserRolePO{}).Error
	})
}

// FindByID 按 ID 查询（不含密码哈希；改密校验旧密码请用 FindByIDWithPassword）
func (r *repo) FindByID(ctx context.Context, id uint) (*user.User, error) {
	var po model.UserPO
	if err := r.data.DB.WithContext(ctx).Model(&model.UserPO{}).Omit("password").First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	u := model.UserFromPO(&po)
	if err := r.loadRoles(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// FindByIDWithPassword 按 ID 查询并携带密码哈希（校验旧密码场景专用）
func (r *repo) FindByIDWithPassword(ctx context.Context, id uint) (*user.User, error) {
	var po model.UserPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	u := model.UserFromPO(&po)
	if err := r.loadRoles(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (r *repo) FindByUsername(ctx context.Context, username string) (*user.User, error) {
	var po model.UserPO
	if err := r.data.DB.WithContext(ctx).Where("username = ?", username).First(&po).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.UserFromPO(&po), nil
}

func (r *repo) List(ctx context.Context, q user.Query, page, pageSize int) ([]*user.User, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.UserPO{})
	if q.Username != "" {
		// 转义用户输入中的 LIKE 通配符，防止 %/_ 改变匹配语义（通配符注入）
		tx = tx.Where("username LIKE ? ESCAPE '/'", security.EscapeLike(q.Username)+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.UserPO
	// 列表查询不带密码哈希
	if err := tx.Omit("password").Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*user.User, 0, len(pos))
	for i := range pos {
		out = append(out, model.UserFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *repo) SetRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.replaceRoles(tx, userID, roleIDs)
	})
}
