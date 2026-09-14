// Package role 角色仓储 GORM 实现
package role

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/internal/biz/role"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

type repo struct {
	data *data.Data
}

// NewRepo 创建角色仓储
func NewRepo(d *data.Data) role.Repo { return &repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return role.ErrRoleNotFound
	}
	return err
}

func (r *repo) loadPermissions(ctx context.Context, ro *role.Role) error {
	var ps []model.RolePermissionPO
	if err := r.data.DB.WithContext(ctx).Where("role_id = ?", ro.ID).Find(&ps).Error; err != nil {
		return err
	}
	for _, x := range ps {
		ro.PermissionIDs = append(ro.PermissionIDs, x.PermissionID)
	}
	return nil
}

func (r *repo) Create(ctx context.Context, ro *role.Role) error {
	po := model.RoleToPO(ro)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	// 回写自增主键与时间戳，供应用层返回给前端
	ro.ID, ro.CreatedAt, ro.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) Update(ctx context.Context, ro *role.Role) error {
	return r.data.DB.WithContext(ctx).Model(&model.RolePO{}).Where("id = ?", ro.ID).
		Updates(map[string]interface{}{"name": ro.Name, "remark": ro.Remark}).Error
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.RolePO{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return role.ErrRoleNotFound
		}
		if err := tx.Where("role_id = ?", id).Delete(&model.RolePermissionPO{}).Error; err != nil {
			return err
		}
		return tx.Where("role_id = ?", id).Delete(&model.UserRolePO{}).Error
	})
}

func (r *repo) FindByID(ctx context.Context, id uint) (*role.Role, error) {
	var po model.RolePO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	ro := model.RoleFromPO(&po)
	if err := r.loadPermissions(ctx, ro); err != nil {
		return nil, err
	}
	return ro, nil
}

// FindNamesByIDs 按角色 ID 列表查角色名（按 id 排序；个人中心展示用）
func (r *repo) FindNamesByIDs(ctx context.Context, ids []uint) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var names []string
	err := r.data.DB.WithContext(ctx).Model(&model.RolePO{}).
		Where("id IN ?", ids).Order("id").Pluck("name", &names).Error
	return names, err
}

// CountUsers 统计角色下的用户数量（删除保护用）
func (r *repo) CountUsers(ctx context.Context, roleID uint) (int64, error) {
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.UserRolePO{}).
		Where("role_id = ?", roleID).Count(&n).Error
	return n, err
}

func (r *repo) List(ctx context.Context, q role.Query, page, pageSize int) ([]*role.Role, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.RolePO{})
	if q.Name != "" {
		// 转义用户输入中的 LIKE 通配符，防止 %/_ 改变匹配语义（通配符注入）
		tx = tx.Where("name LIKE ? ESCAPE '/'", security.EscapeLike(q.Name)+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.RolePO
	if err := tx.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*role.Role, 0, len(pos))
	for i := range pos {
		out = append(out, model.RoleFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *repo) SetPermissions(ctx context.Context, roleID uint, permissionIDs []uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermissionPO{}).Error; err != nil {
			return err
		}
		for _, pid := range permissionIDs {
			if err := tx.Create(&model.RolePermissionPO{RoleID: roleID, PermissionID: pid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
