// Package appuser 应用用户仓储 GORM 实现：用户同步读写 + 租户关联先删后插替换。
package appuser

import (
	"context"
	"errors"
	"strings"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

// mapErr 哨兵错误映射：记录不存在 / 唯一索引冲突（sqlite/mysql/postgres 文案判断）
func mapErr(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return bizappuser.ErrAppUserNotFound
	case isUniqueViolation(err):
		return bizappuser.ErrDuplicateUsername
	}
	return err
}

// isUniqueViolation 各数据库唯一约束冲突文案判断：
// MySQL Error 1062、Postgres 23505（duplicate key value）、SQLite UNIQUE constraint failed
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "UNIQUE constraint failed")
}

type repo struct {
	data *data.Data
}

// NewRepo 创建应用用户仓储
func NewRepo(d *data.Data) bizappuser.Repo { return &repo{data: d} }

func (r *repo) Create(ctx context.Context, u *bizappuser.AppUser) error {
	po := model.AppUserToPO(u)
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(po).Error; err != nil {
			return mapErr(err)
		}
		u.ID = po.ID
		u.CreatedAt, u.UpdatedAt = po.CreatedAt, po.UpdatedAt
		return replaceTenants(tx, u.ID, u.TenantIDs)
	})
}

func (r *repo) Update(ctx context.Context, u *bizappuser.AppUser) error {
	po := model.AppUserToPO(u)
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 只更新基础资料字段；密码一律走 UpdatePassword，避免无哈希查询后被意外清空
		res := tx.Model(&model.AppUserPO{}).Where("id = ?", u.ID).
			Updates(map[string]interface{}{
				"nickname": po.Nickname, "phone": po.Phone, "email": po.Email, "status": po.Status,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return bizappuser.ErrAppUserNotFound
		}
		return replaceTenants(tx, u.ID, u.TenantIDs)
	})
}

// replaceTenants 事务内全量替换租户关联（先删后插；复合唯一索引下软删残留会阻碍重新绑定，故物理删除）
func replaceTenants(tx *gorm.DB, appUserID uint, tenantIDs []uint) error {
	if err := tx.Unscoped().Where("app_user_id = ?", appUserID).Delete(&model.AppUserTenantPO{}).Error; err != nil {
		return err
	}
	for _, tid := range tenantIDs {
		if err := tx.Create(&model.AppUserTenantPO{AppUserID: appUserID, TenantID: tid}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *repo) ReplaceTenants(ctx context.Context, id uint, tenantIDs []uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return replaceTenants(tx, id, tenantIDs)
	})
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.AppUserPO{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return bizappuser.ErrAppUserNotFound
		}
		// 关联随用户一并清理（物理删除，避免阻断租户删除保护计数）
		return tx.Unscoped().Where("app_user_id = ?", id).Delete(&model.AppUserTenantPO{}).Error
	})
}

func (r *repo) UpdatePassword(ctx context.Context, id uint, passwordHash string) error {
	return r.data.DB.WithContext(ctx).Model(&model.AppUserPO{}).Where("id = ?", id).
		Updates(map[string]interface{}{"password_hash": passwordHash}).Error
}

func (r *repo) Get(ctx context.Context, id uint) (*bizappuser.AppUser, error) {
	var po model.AppUserPO
	// 详情查询不带密码哈希
	if err := r.data.DB.WithContext(ctx).Model(&model.AppUserPO{}).Omit("password_hash").First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	u := model.AppUserFromPO(&po)
	if err := r.loadTenants(ctx, []*bizappuser.AppUser{u}); err != nil {
		return nil, err
	}
	return u, nil
}

// GetByUsername 按用户名查询并携带密码哈希（登录/改密校验专用）
func (r *repo) GetByUsername(ctx context.Context, username string) (*bizappuser.AppUser, error) {
	var po model.AppUserPO
	if err := r.data.DB.WithContext(ctx).Where("username = ?", username).First(&po).Error; err != nil {
		return nil, mapErr(err)
	}
	u := model.AppUserFromPO(&po)
	if err := r.loadTenants(ctx, []*bizappuser.AppUser{u}); err != nil {
		return nil, err
	}
	return u, nil
}

func (r *repo) List(ctx context.Context, q bizappuser.Query, page, pageSize int) ([]*bizappuser.AppUser, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.AppUserPO{})
	// 关键词全模糊匹配用户名/昵称；转义 LIKE 通配符防通配符注入
	if q.Keyword != "" {
		kw := "%" + security.EscapeLike(q.Keyword) + "%"
		tx = tx.Where("(username LIKE ? ESCAPE '/' OR nickname LIKE ? ESCAPE '/')", kw, kw)
	}
	if q.Phone != "" {
		tx = tx.Where("phone = ?", q.Phone)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	// 按租户筛选：关联表子查询（软删关联由 GORM 默认条件剔除）
	if q.TenantID != nil {
		tx = tx.Where("id IN (SELECT app_user_id FROM app_user_tenants WHERE tenant_id = ? AND deleted_at IS NULL)", *q.TenantID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.AppUserPO
	// 列表查询不带密码哈希
	if err := tx.Omit("password_hash").Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*bizappuser.AppUser, 0, len(pos))
	for i := range pos {
		out = append(out, model.AppUserFromPO(&pos[i]))
	}
	if err := r.loadTenants(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// loadTenants 批量聚合租户关联：先查关联表，再按 tenant_id 批量查租户名称，内存组装（避免 N+1）
func (r *repo) loadTenants(ctx context.Context, users []*bizappuser.AppUser) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	var rels []model.AppUserTenantPO
	if err := r.data.DB.WithContext(ctx).Where("app_user_id IN ?", ids).Find(&rels).Error; err != nil {
		return err
	}
	tenantIDs := make([]uint, 0, len(rels))
	seen := map[uint]bool{}
	for _, rel := range rels {
		if !seen[rel.TenantID] {
			seen[rel.TenantID] = true
			tenantIDs = append(tenantIDs, rel.TenantID)
		}
	}
	names := map[uint]string{}
	if len(tenantIDs) > 0 {
		var tenants []model.TenantPO
		if err := r.data.DB.WithContext(ctx).Where("id IN ?", tenantIDs).Find(&tenants).Error; err != nil {
			return err
		}
		for _, t := range tenants {
			names[t.ID] = t.Name
		}
	}
	byUser := map[uint][]uint{}
	for _, rel := range rels {
		byUser[rel.AppUserID] = append(byUser[rel.AppUserID], rel.TenantID)
	}
	for _, u := range users {
		u.TenantIDs = byUser[u.ID]
		u.TenantNames = []string{}
		for _, tid := range u.TenantIDs {
			if name, ok := names[tid]; ok {
				u.TenantNames = append(u.TenantNames, name)
			}
		}
	}
	return nil
}
