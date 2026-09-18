// Package tenant 租户仓储 GORM 实现
package tenant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

// mapErr 哨兵错误映射：记录不存在 / 唯一索引冲突（sqlite/mysql/postgres 文案判断）；
// name 与 code 均有唯一索引，按报错信息中的索引/列名区分（idx_tenants_name 或 tenants.name）
func mapErr(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return biztenant.ErrTenantNotFound
	case isUniqueViolation(err):
		if msg := err.Error(); strings.Contains(msg, "tenants_name") || strings.Contains(msg, "tenants.name") {
			return biztenant.ErrDuplicateTenantName
		}
		return biztenant.ErrDuplicateTenantCode
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

// NewRepo 创建租户仓储
func NewRepo(d *data.Data) biztenant.Repo { return &repo{data: d} }

func (r *repo) Create(ctx context.Context, t *biztenant.Tenant) error {
	po := model.TenantToPO(t)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return mapErr(err)
	}
	t.ID = po.ID
	t.CreatedAt, t.UpdatedAt = po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) Update(ctx context.Context, t *biztenant.Tenant) error {
	po := model.TenantToPO(t)
	// code 创建后不可改，仅更新基础资料、平台映射与状态
	res := r.data.DB.WithContext(ctx).Model(&model.TenantPO{}).Where("id = ?", t.ID).
		Updates(map[string]interface{}{
			"name": po.Name, "contact_name": po.ContactName, "contact_phone": po.ContactPhone,
			"remark": po.Remark, "status": po.Status, "platform_id": po.PlatformID,
		})
	if res.Error != nil {
		return mapErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return biztenant.ErrTenantNotFound
	}
	return nil
}

// Delete 删除租户：存在关联应用用户（app_user_tenants 有直接计数，避免跨上下文依赖）时拒绝。
// 软删行仍占用本地唯一索引（name/code 单列唯一）——删除时改写释放槽位（后缀 #del#<id>），
// 否则本地重建同 code/name 租户会误报重复（进而触发 Create 回滚平台侧的连锁）
func (r *repo) Delete(ctx context.Context, id uint) error {
	var refCnt int64
	if err := r.data.DB.WithContext(ctx).Model(&model.AppUserTenantPO{}).
		Where("tenant_id = ?", id).Count(&refCnt).Error; err != nil {
		return err
	}
	if refCnt > 0 {
		return biztenant.ErrTenantInUse
	}
	var po model.TenantPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return mapErr(err)
	}
	res := r.data.DB.WithContext(ctx).Model(&model.TenantPO{}).Where("id = ?", id).
		Updates(map[string]any{
			"code":       freeUniqueSlot(po.Code, id),
			"name":       freeUniqueSlot(po.Name, id),
			"deleted_at": time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biztenant.ErrTenantNotFound
	}
	return nil
}

// freeUniqueSlot 软删时释放唯一槽位：<原值>#del#<id>；超列宽（64 字符）按 rune 截头保留后缀
func freeUniqueSlot(v string, id uint) string {
	const width = 64
	suffix := fmt.Sprintf("#del#%d", id)
	if utf8.RuneCountInString(v)+len(suffix) <= width {
		return v + suffix
	}
	return string([]rune(v)[:width-len(suffix)]) + suffix
}

func (r *repo) Get(ctx context.Context, id uint) (*biztenant.Tenant, error) {
	var po model.TenantPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.TenantFromPO(&po), nil
}

func (r *repo) List(ctx context.Context, q biztenant.Query, page, pageSize int) ([]*biztenant.Tenant, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.TenantPO{})
	// 按字段独立全模糊匹配；转义用户输入中的 LIKE 通配符，防止 %/_ 改变匹配语义（通配符注入）
	if q.Name != "" {
		tx = tx.Where("name LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Name)+"%")
	}
	if q.Code != "" {
		tx = tx.Where("code LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Code)+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.TenantPO
	if err := tx.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biztenant.Tenant, 0, len(pos))
	for i := range pos {
		out = append(out, model.TenantFromPO(&pos[i]))
	}
	return out, total, nil
}

// ListUnsynced 列出尚未与平台同步（platform_id=0）的租户（存量补链用）
func (r *repo) ListUnsynced(ctx context.Context) ([]*biztenant.Tenant, error) {
	var pos []model.TenantPO
	if err := r.data.DB.WithContext(ctx).
		Where("platform_id = 0").Order("id").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*biztenant.Tenant, 0, len(pos))
	for i := range pos {
		out = append(out, model.TenantFromPO(&pos[i]))
	}
	return out, nil
}
