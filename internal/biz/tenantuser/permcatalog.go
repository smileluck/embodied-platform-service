// 租户门户权限码注册表（商户侧事实源）。平台目录自 2026-10-09 动态化：本表在服务启动时
// 经开放面 PUT /open-api/v1/tenant-user-perms 向平台同步（幂等、只增不删、按商户隔离），
// 有效目录 = 平台 base 目录 ∪ 本表。新增门户功能权限码：本表加一行 + 路由挂载引用常量
// （RequireTenantPerm）+ 前端 i18n 文案（tenantRole.perm/permGroup），同发版即生效，无需平台改码。
package tenantuser

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

// 数据集域（平台 base 已撤下，由本注册表带回——dataset 为商户端业务域）
const (
	PermDatasetList   = "dataset:list"
	PermDatasetCreate = "dataset:create"
	PermDatasetUpload = "dataset:upload"
	PermDatasetDelete = "dataset:delete"
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
// 初始 27 码对齐平台 2026-10-08 静态目录（含平台 2026-10-09 撤下的 dataset 域）。
var Catalog = []PermDefView{
	{Code: PermDeviceList, Group: "device"},
	{Code: PermDeviceCreate, Group: "device"},
	{Code: PermDeviceUpdate, Group: "device"},
	{Code: PermDeviceDelete, Group: "device"},
	{Code: PermAlarmList, Group: "alarm"},
	{Code: PermAlarmUpdate, Group: "alarm"},
	{Code: PermAlarmDelete, Group: "alarm"},
	{Code: PermDatasetList, Group: "dataset"},
	{Code: PermDatasetCreate, Group: "dataset"},
	{Code: PermDatasetUpload, Group: "dataset"},
	{Code: PermDatasetDelete, Group: "dataset"},
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
