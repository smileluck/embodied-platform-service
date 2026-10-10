// Package tenantdept 租户部门本地仓储 GORM 实现（2026-10-10 租户内部数据隔离）。
// tenant_id 为平台租户 ID、user_id 为平台 tenant_user ID（无外键引用，身份事实源在平台）。
package tenantdept

import (
	"context"
	"errors"

	biztenantdept "github.com/smilex/smilex-admin-gin/internal/biz/tenantdept"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"gorm.io/gorm"
)

// mapDeptErr 部门侧哨兵错误映射：记录不存在 / (tenant_id, code) 复合唯一冲突
func mapDeptErr(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return biztenantdept.ErrDeptNotFound
	case data.IsUniqueViolation(err):
		return biztenantdept.ErrDuplicateDeptCode
	}
	return err
}

type Repo struct {
	data *data.Data
}

// NewRepo 创建租户部门仓储（wire provider）
func NewRepo(d *data.Data) *Repo { return &Repo{data: d} }

func (r *Repo) Create(ctx context.Context, d *biztenantdept.TenantDept) error {
	po := toPO(d)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return mapDeptErr(err)
	}
	d.ID, d.CreatedAt, d.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

// Update 更新部门（id/tenant_id/code 创建后不可改，仅更新父级/名称/排序/备注）
func (r *Repo) Update(ctx context.Context, d *biztenantdept.TenantDept) error {
	res := r.data.DB.WithContext(ctx).Model(&model.TenantDeptPO{}).Where("id = ?", d.ID).
		Updates(map[string]interface{}{
			"parent_id": d.ParentID, "name": d.Name, "sort": d.Sort, "remark": d.Remark,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biztenantdept.ErrDeptNotFound
	}
	return nil
}

// Delete 删除部门：事务内软删（墓碑改写 code 释放 (tenant_id, code) 唯一槽位）+
// 物理清理成员绑定（usecase 层已拒绝有子部门，成员归属随部门消亡失义）
func (r *Repo) Delete(ctx context.Context, id uint) error {
	var po model.TenantDeptPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return mapDeptErr(err)
	}
	now := r.data.DB.NowFunc()
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.TenantDeptPO{}).Where("id = ?", id).
			Updates(map[string]interface{}{
				"code":       data.TombstoneCode(po.Code, id, now),
				"deleted_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return biztenantdept.ErrDeptNotFound
		}
		return tx.Where("dept_id = ?", id).Delete(&model.TenantUserDeptBindPO{}).Error
	})
}

func (r *Repo) Get(ctx context.Context, id uint) (*biztenantdept.TenantDept, error) {
	var po model.TenantDeptPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapDeptErr(err)
	}
	return fromPO(&po), nil
}

// List 租户下全量部门平表（parent+sort 排序稳定呈现，树形组装在前端）
func (r *Repo) List(ctx context.Context, tenantID uint) ([]*biztenantdept.TenantDept, error) {
	var pos []model.TenantDeptPO
	if err := r.data.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("parent_id, sort, id").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biztenantdept.TenantDept, 0, len(pos))
	for i := range pos {
		out = append(out, fromPO(&pos[i]))
	}
	return out, nil
}

func (r *Repo) CountChildren(ctx context.Context, parentID uint) (int64, error) {
	var cnt int64
	err := r.data.DB.WithContext(ctx).Model(&model.TenantDeptPO{}).
		Where("parent_id = ?", parentID).Count(&cnt).Error
	return cnt, err
}

// ReplaceUserDepts 事务内全量替换用户部门绑定（先删后插；复合主键下残留会阻碍重新分配）
func (r *Repo) ReplaceUserDepts(ctx context.Context, userID uint, deptIDs []uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.TenantUserDeptBindPO{}).Error; err != nil {
			return err
		}
		for _, did := range deptIDs {
			if err := tx.Create(&model.TenantUserDeptBindPO{UserID: userID, DeptID: did}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteUserDepts 清理用户全部部门绑定（用户删号时级联——残留会虚增 member_count；
// 幂等，无绑定时删 0 行）
func (r *Repo) DeleteUserDepts(ctx context.Context, userID uint) error {
	return r.data.DB.WithContext(ctx).Where("user_id = ?", userID).
		Delete(&model.TenantUserDeptBindPO{}).Error
}

// UserDeptIDs 批量取用户部门绑定（user_id -> dept_ids；user_ids 为平台 tenant_user ID）
func (r *Repo) UserDeptIDs(ctx context.Context, userIDs []uint) (map[uint][]uint, error) {
	out := make(map[uint][]uint, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var binds []model.TenantUserDeptBindPO
	if err := r.data.DB.WithContext(ctx).Where("user_id IN ?", userIDs).Order("dept_id").Find(&binds).Error; err != nil {
		return nil, err
	}
	for _, b := range binds {
		out[b.UserID] = append(out[b.UserID], b.DeptID)
	}
	return out, nil
}

// UserIDsByDepts 取部门集合直属成员用户 ID 去重集合（后代展开由用例层完成）
func (r *Repo) UserIDsByDepts(ctx context.Context, deptIDs []uint) ([]uint, error) {
	if len(deptIDs) == 0 {
		return []uint{}, nil
	}
	var ids []uint
	if err := r.data.DB.WithContext(ctx).Model(&model.TenantUserDeptBindPO{}).
		Where("dept_id IN ?", deptIDs).Distinct().Order("user_id").Pluck("user_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// MemberCounts 租户下各部门直属成员数（dept_id -> count；软删部门的绑定已在删除时清理）
func (r *Repo) MemberCounts(ctx context.Context, tenantID uint) (map[uint]int64, error) {
	var rows []struct {
		DeptID uint
		Cnt    int64
	}
	err := r.data.DB.WithContext(ctx).Model(&model.TenantUserDeptBindPO{}).
		Select("dept_id, COUNT(*) AS cnt").
		Where("dept_id IN (SELECT id FROM tenant_depts WHERE tenant_id = ? AND deleted_at IS NULL)", tenantID).
		Group("dept_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]int64, len(rows))
	for _, row := range rows {
		out[row.DeptID] = row.Cnt
	}
	return out, nil
}

// DeptNamesByIDs 批量取部门名（dept_id -> name；软删部门名不返回——绑定已随删除清理，
// 残留引用按缺名处理）
func (r *Repo) DeptNamesByIDs(ctx context.Context, deptIDs []uint) (map[uint]string, error) {
	out := make(map[uint]string, len(deptIDs))
	if len(deptIDs) == 0 {
		return out, nil
	}
	var pos []model.TenantDeptPO
	if err := r.data.DB.WithContext(ctx).Select("id", "name").Where("id IN ?", deptIDs).Find(&pos).Error; err != nil {
		return nil, err
	}
	for i := range pos {
		out[pos[i].ID] = pos[i].Name
	}
	return out, nil
}

func toPO(d *biztenantdept.TenantDept) *model.TenantDeptPO {
	return &model.TenantDeptPO{
		ID: d.ID, TenantID: d.TenantID, ParentID: d.ParentID,
		Name: d.Name, Code: d.Code, Sort: d.Sort, Remark: d.Remark,
	}
}

func fromPO(p *model.TenantDeptPO) *biztenantdept.TenantDept {
	return &biztenantdept.TenantDept{
		ID: p.ID, TenantID: p.TenantID, ParentID: p.ParentID,
		Name: p.Name, Code: p.Code, Sort: p.Sort, Remark: p.Remark,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}
