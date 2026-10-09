// 租户门户权限码注册表（本地唯一真源，2026-10-09 起随租户 RBAC 本地化——不再向平台同步，
// 平台目录动态化链路已退役）。新增门户功能权限码：本表加一行 + 路由挂载引用常量
// （RequireTenantPerm）+ 前端 i18n 文案（tenantRole.perm/permGroup），同发版即生效。
package tenantuser

import (
	"errors"
	"sort"
	"strings"
)

// MaxPerms 单角色权限点数上限（防滥用与误配巨集）
const MaxPerms = 128

// ErrInvalidPerm 权限点不在本地注册表目录内或超上限
var ErrInvalidPerm = errors.New("权限点无效")

// 设备域
const (
	PermDeviceList   = "device:list"
	PermDeviceCreate = "device:create"
	PermDeviceUpdate = "device:update"
	PermDeviceDelete = "device:delete"
)

// 告警域
const (
	PermAlarmList   = "alarm:list"
	PermAlarmUpdate = "alarm:update"
	PermAlarmDelete = "alarm:delete"
)

// 应用用户域（租户内 C 端终端用户运营）
const (
	PermAppUserList          = "appuser:list"
	PermAppUserCreate        = "appuser:create"
	PermAppUserUpdate        = "appuser:update"
	PermAppUserDelete        = "appuser:delete"
	PermAppUserResetPassword = "appuser:resetPassword"
)

// 成员域（租户内成员与角色自管）
const (
	PermMemberUserList          = "member:user:list"
	PermMemberUserCreate        = "member:user:create"
	PermMemberUserUpdate        = "member:user:update"
	PermMemberUserDelete        = "member:user:delete"
	PermMemberUserResetPassword = "member:user:resetPassword"
	PermMemberUserSetRoles      = "member:user:setRoles"
	PermMemberRoleList          = "member:role:list"
	PermMemberRoleCreate        = "member:role:create"
	PermMemberRoleUpdate        = "member:role:update"
	PermMemberRoleDelete        = "member:role:delete"
	PermMemberRoleSetPerms      = "member:role:setPerms"
)

// Catalog 权限码注册表（展示顺序即配权界面分组顺序；与前端 tenantRole.permGroup 分组对齐）。
// 初始 23 码对齐平台 2026-10-09 目录（dataset 域随平台 a5c0f644 撤下：数据集上传已收敛为
// 设备端专用+管理面，租户门户无任何数据集功能，权限码不挂任何路由）。
var Catalog = []PermDefView{
	{Code: PermDeviceList, Group: "device"},
	{Code: PermDeviceCreate, Group: "device"},
	{Code: PermDeviceUpdate, Group: "device"},
	{Code: PermDeviceDelete, Group: "device"},
	{Code: PermAlarmList, Group: "alarm"},
	{Code: PermAlarmUpdate, Group: "alarm"},
	{Code: PermAlarmDelete, Group: "alarm"},
	{Code: PermAppUserList, Group: "appuser"},
	{Code: PermAppUserCreate, Group: "appuser"},
	{Code: PermAppUserUpdate, Group: "appuser"},
	{Code: PermAppUserDelete, Group: "appuser"},
	{Code: PermAppUserResetPassword, Group: "appuser"},
	{Code: PermMemberUserList, Group: "member"},
	{Code: PermMemberUserCreate, Group: "member"},
	{Code: PermMemberUserUpdate, Group: "member"},
	{Code: PermMemberUserDelete, Group: "member"},
	{Code: PermMemberUserResetPassword, Group: "member"},
	{Code: PermMemberUserSetRoles, Group: "member"},
	{Code: PermMemberRoleList, Group: "member"},
	{Code: PermMemberRoleCreate, Group: "member"},
	{Code: PermMemberRoleUpdate, Group: "member"},
	{Code: PermMemberRoleDelete, Group: "member"},
	{Code: PermMemberRoleSetPerms, Group: "member"},
}

// knownPermCodes 目录码索引（NormalizePerms 校验用，包初始化构建）
var knownPermCodes = func() map[string]bool {
	m := make(map[string]bool, len(Catalog))
	for _, d := range Catalog {
		m[d.Code] = true
	}
	return m
}()

// NormalizePerms 校验并归一化待保存的权限点集合（本地注册表目录口径）：每项须为目录精确码；
// 去重、按字典序稳定排序；超过 MaxPerms 拒绝。返回可落库集合。
func NormalizePerms(perms []string) ([]string, error) {
	seen := make(map[string]bool, len(perms))
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		if !knownPermCodes[p] {
			return nil, ErrInvalidPerm
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) > MaxPerms {
		return nil, ErrInvalidPerm
	}
	sort.Strings(out)
	return out, nil
}

// PermAllowed 判断权限码集合是否覆盖单个操作码（精确匹配，无通配形态）
func PermAllowed(perms []string, code string) bool {
	for _, p := range perms {
		if p == code {
			return true
		}
	}
	return false
}
