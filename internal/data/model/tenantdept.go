// 租户部门本地两表（2026-10-10 租户内部数据隔离基础能力，对齐 tenant_roles 2026-10-09
// 自平台下沉先例：账号身份事实源在平台，组织架构与归属绑定放本地）。
// tenant_id 一律为平台租户 ID、user_id 一律为平台 tenant_user ID（无外键引用——
// 与 tenants.platform_id 投影同口径）。
package model

import (
	"time"

	"gorm.io/gorm"
)

// TenantDeptPO 租户部门（树形自引用 parent_id，0=根；(tenant_id, code) 复合唯一——
// 同一字段组合仅参与一个命名索引可走 GORM tag；软删留痕，删除时墓碑改写 code 释放槽位）
type TenantDeptPO struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  uint   `gorm:"uniqueIndex:uk_tenant_depts_tenant_code;index"` // 平台租户 ID
	ParentID  uint   `gorm:"index;default:0"`                               // 父部门 ID，0=根
	Name      string `gorm:"size:64"`
	Code      string `gorm:"size:64;uniqueIndex:uk_tenant_depts_tenant_code"`
	Sort      int
	Remark    string `gorm:"size:255"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TenantUserDeptBindPO 用户-部门绑定（复合主键，多部门归属；替换时物理删除）。
// 表名已核对不在 migrateLegacy 清理名单（退役的是 tenant_user_roles）
type TenantUserDeptBindPO struct {
	UserID    uint `gorm:"primaryKey"` // 平台 tenant_user ID
	DeptID    uint `gorm:"primaryKey"`
	CreatedAt time.Time
}

func (TenantDeptPO) TableName() string         { return "tenant_depts" }
func (TenantUserDeptBindPO) TableName() string { return "tenant_user_dept_binds" }
