// Package admission 准入限界上下文 —— 领域层。
//
// 平台（embodied-platform）是唯一身份源：账号/密码/会话全部在平台侧，
// 登录由本系统前端直调平台完成（token 双用）。本上下文只维护
// 「平台用户 → 本系统准入投影」：一行一人，platform_user_id 唯一，
// 含准入开关与本地角色绑定；首登懒建、默认关闭（对应平台统一账号决策
// 「准入在业务平台侧」）。平台账号禁用=全局失效（登录不了平台即进不来本系统）；
// 投影禁用/删除只影响本系统（三层开关中的「准入=项目×个人」层）。
package admission

import "time"

// Projection 平台用户在本系统的准入投影
type Projection struct {
	ID             uint      `json:"id"`
	PlatformUserID uint      `json:"platform_user_id"` // 平台用户 ID（唯一）
	Username       string    `json:"username"`         // 平台用户名快照
	Nickname       string    `json:"nickname"`         // 平台昵称快照（展示用，以平台为准）
	Email          string    `json:"email"`
	Enabled        bool      `json:"enabled"`  // 准入开关：false 时无法访问本系统
	RoleIDs        []uint    `json:"role_ids"` // 本地角色绑定
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// MerchantAdminRoleID 商户管理员内置角色固定 ID（平台侧商户成员的商户管理员
// 标记驱动绑定/解绑，是本系统唯一的内置角色；锁定、不参与手工分配，
// 见 usecase 的投影逻辑）
const MerchantAdminRoleID uint = 2

// LockedRoleIDs 内置锁定角色集（商户管理员=平台标记驱动；
// 手工全量分配角色时保留现持有绑定，不接受增删）
var LockedRoleIDs = []uint{MerchantAdminRoleID}

// IsLockedRole 是否内置锁定角色
func IsLockedRole(id uint) bool {
	return id == MerchantAdminRoleID
}

// MerchantRef 本人已准入的商户引用（来自平台 /auth/profile 的 user.merchants；
// IsAdmin 即平台的商户管理员标记——本系统据此把标记投影为本地商户管理员角色）
type MerchantRef struct {
	ID      uint   `json:"id"`
	Code    string `json:"code"`
	IsAdmin bool   `json:"is_admin"`
}
