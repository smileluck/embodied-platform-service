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

// SuperRoleID 本地超管角色固定 ID（种子数据约定，与角色上下文的锁定规则共用）
const SuperRoleID uint = 1
