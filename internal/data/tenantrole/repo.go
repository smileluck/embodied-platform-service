// Package tenantrole 租户 RBAC 本地仓储 GORM 实现（2026-10-09 自平台下沉）。
// tenant_id 为平台租户 ID、user_id 为平台 tenant_user ID（无外键引用，身份事实源在平台）。
package tenantrole

import (
	"context"
	"errors"

	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

// mapRoleErr 角色侧哨兵错误映射：记录不存在 / (tenant_id, code) 复合唯一冲突
func mapRoleErr(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return biztenantuser.ErrTenantRoleNotFound
	case data.IsUniqueViolation(err):
		return biztenantuser.ErrDuplicateRoleCode
	}
	return err
}

type Repo struct {
	data *data.Data
}

// NewRepo 创建租户角色仓储（wire provider）
func NewRepo(d *data.Data) *Repo { return &Repo{data: d} }

func (r *Repo) Create(ctx context.Context, role *biztenantuser.TenantRole) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		po := toPO(role)
		if err := tx.Create(po).Error; err != nil {
			return mapRoleErr(err)
		}
		role.ID = po.ID
		role.CreatedAt, role.UpdatedAt = po.CreatedAt, po.UpdatedAt
		return replacePerms(tx, role.ID, role.PermCodes)
	})
}

func (r *Repo) Update(ctx context.Context, role *biztenantuser.TenantRole) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// code/tenant_id 创建后不可改，仅更新名称/备注；权限点走 replacePerms 全量替换
		res := tx.Model(&model.TenantRolePO{}).Where("id = ?", role.ID).
			Updates(map[string]interface{}{"name": role.Name, "remark": role.Remark})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return biztenantuser.ErrTenantRoleNotFound
		}
		return replacePerms(tx, role.ID, role.PermCodes)
	})
}

// replacePerms 事务内全量替换权限点（先删后插；复合主键无软删）
func replacePerms(tx *gorm.DB, roleID uint, permCodes []string) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.TenantRolePermPO{}).Error; err != nil {
		return err
	}
	for _, code := range permCodes {
		if err := tx.Create(&model.TenantRolePermPO{RoleID: roleID, PermCode: code}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ReplaceUserRoles 事务内全量替换用户角色绑定（先删后插；复合主键下残留会阻碍重新分配）
func (r *Repo) ReplaceUserRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.TenantUserRoleBindPO{}).Error; err != nil {
			return err
		}
		for _, rid := range roleIDs {
			if err := tx.Create(&model.TenantUserRoleBindPO{UserID: userID, RoleID: rid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Delete 删除角色：仍有用户绑定时拒绝（ErrRoleInUse）。软删行仍占 (tenant_id, code)
// 复合唯一索引——墓碑改写 code 释放槽位；权限点关联物理清理
func (r *Repo) Delete(ctx context.Context, id uint) error {
	var bindCnt int64
	if err := r.data.DB.WithContext(ctx).Model(&model.TenantUserRoleBindPO{}).
		Where("role_id = ?", id).Count(&bindCnt).Error; err != nil {
		return err
	}
	if bindCnt > 0 {
		return biztenantuser.ErrRoleInUse
	}
	var po model.TenantRolePO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return mapRoleErr(err)
	}
	now := r.data.DB.NowFunc()
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.TenantRolePO{}).Where("id = ?", id).
			Updates(map[string]interface{}{
				"code":       data.TombstoneCode(po.Code, id, now),
				"deleted_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return biztenantuser.ErrTenantRoleNotFound
		}
		return tx.Where("role_id = ?", id).Delete(&model.TenantRolePermPO{}).Error
	})
}

func (r *Repo) Get(ctx context.Context, id uint) (*biztenantuser.TenantRole, error) {
	var po model.TenantRolePO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapRoleErr(err)
	}
	role := fromPO(&po)
	if err := r.loadPerms(ctx, []*biztenantuser.TenantRole{role}); err != nil {
		return nil, err
	}
	return role, nil
}

func (r *Repo) List(ctx context.Context, q biztenantuser.RoleListParams, page, pageSize int) ([]*biztenantuser.TenantRole, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.TenantRolePO{})
	// 关键词全模糊匹配名称/编码；转义 LIKE 通配符防通配符注入
	if q.Keyword != "" {
		kw := "%" + security.EscapeLike(q.Keyword) + "%"
		tx = tx.Where("(name LIKE ? ESCAPE '/' OR code LIKE ? ESCAPE '/')", kw, kw)
	}
	if q.TenantID != nil {
		tx = tx.Where("tenant_id = ?", *q.TenantID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.TenantRolePO
	// pageSize<=0 表示全量（不分页）
	if pageSize > 0 {
		tx = tx.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	if err := tx.Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biztenantuser.TenantRole, 0, len(pos))
	for i := range pos {
		out = append(out, fromPO(&pos[i]))
	}
	if err := r.loadPerms(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// UserRoleIDs 批量取用户角色绑定（user_id -> role_ids；user_ids 为平台 tenant_user ID）
func (r *Repo) UserRoleIDs(ctx context.Context, userIDs []uint) (map[uint][]uint, error) {
	out := make(map[uint][]uint, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var binds []model.TenantUserRoleBindPO
	if err := r.data.DB.WithContext(ctx).Where("user_id IN ?", userIDs).Order("role_id").Find(&binds).Error; err != nil {
		return nil, err
	}
	for _, b := range binds {
		out[b.UserID] = append(out[b.UserID], b.RoleID)
	}
	return out, nil
}

// ResolvePerms 用户权限码集合：binds → 本租户角色 → role_perms 去重
// （TenantAuth 缓存装载调用，走索引；tenantID 双重收敛防跨租户角色残留）
func (r *Repo) ResolvePerms(ctx context.Context, userID, tenantID uint) ([]string, error) {
	var codes []string
	if err := r.data.DB.WithContext(ctx).Model(&model.TenantRolePermPO{}).
		Where("role_id IN (SELECT b.role_id FROM tenant_user_role_binds b "+
			"JOIN tenant_roles r ON r.id = b.role_id AND r.deleted_at IS NULL AND r.tenant_id = ? "+
			"WHERE b.user_id = ?)", tenantID, userID).
		Distinct().Order("perm_code").Pluck("perm_code", &codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

// loadPerms 批量聚合权限点（避免 N+1）
func (r *Repo) loadPerms(ctx context.Context, roles []*biztenantuser.TenantRole) error {
	if len(roles) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
	}
	var rows []model.TenantRolePermPO
	if err := r.data.DB.WithContext(ctx).Where("role_id IN ?", ids).Order("perm_code").Find(&rows).Error; err != nil {
		return err
	}
	byRole := map[uint][]string{}
	for _, row := range rows {
		byRole[row.RoleID] = append(byRole[row.RoleID], row.PermCode)
	}
	for _, role := range roles {
		role.PermCodes = byRole[role.ID]
		if role.PermCodes == nil {
			role.PermCodes = []string{}
		}
	}
	return nil
}

func toPO(r *biztenantuser.TenantRole) *model.TenantRolePO {
	return &model.TenantRolePO{
		ID: r.ID, TenantID: r.TenantID, Name: r.Name, Code: r.Code, Remark: r.Remark,
	}
}

func fromPO(p *model.TenantRolePO) *biztenantuser.TenantRole {
	return &biztenantuser.TenantRole{
		ID: p.ID, TenantID: p.TenantID, Name: p.Name, Code: p.Code, Remark: p.Remark,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}
