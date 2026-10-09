// 租户 RBAC 本地三表（2026-10-09 自平台下沉：认证留平台 tenant_users，授权本地）。
// tenant_id 一律为平台租户 ID、user_id 一律为平台 tenant_user ID（无外键引用——
// 账号身份事实源在平台，本地只持授权绑定；与 tenants.platform_id 投影同口径）。
package model

import (
	"time"

	"gorm.io/gorm"
)

// TenantRolePO 租户角色（(tenant_id, code) 复合唯一——同一字段组合仅参与一个命名索引可走
// GORM tag；软删留痕，删除时墓碑改写 code 释放槽位）
type TenantRolePO struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  uint   `gorm:"uniqueIndex:uk_tenant_roles_tenant_code;index"` // 平台租户 ID
	Name      string `gorm:"size:64"`
	Code      string `gorm:"size:64;uniqueIndex:uk_tenant_roles_tenant_code"`
	Remark    string `gorm:"size:255"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TenantRolePermPO 角色-权限点关联（复合主键；权限码来自本地注册表 biz/tenantuser/permcatalog.go；
// 替换权限点时物理删除避免主键残留阻碍重赋）
type TenantRolePermPO struct {
	RoleID    uint   `gorm:"primaryKey"`
	PermCode  string `gorm:"primaryKey;size:64"`
	CreatedAt time.Time
}

// TenantUserRoleBindPO 用户-角色绑定（复合主键，替换时物理删除）。
// 表名避开 tenant_user_roles——该旧表（app_user × 租户双角色时代）仍被 migrateLegacy
// 每次启动 HasTable→DropTable 幂等清理，同名新表会被反复清空。
type TenantUserRoleBindPO struct {
	UserID    uint `gorm:"primaryKey"` // 平台 tenant_user ID
	RoleID    uint `gorm:"primaryKey"`
	CreatedAt time.Time
}

func (TenantRolePO) TableName() string        { return "tenant_roles" }
func (TenantRolePermPO) TableName() string    { return "tenant_role_perms" }
func (TenantUserRoleBindPO) TableName() string { return "tenant_user_role_binds" }
