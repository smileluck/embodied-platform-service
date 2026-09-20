package server

import (
	"errors"

	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	bizmonitor "github.com/smilex/smilex-admin-gin/internal/biz/monitor"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/biz/role"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// errKeys 业务哨兵错误 -> 语言包 key 注册表（errors.Is 匹配，顺序即优先级）。
// 带参数的错误（file.too_large、blacklist.ip_banned）不在此列，由调用点带参渲染。
var errKeys = []struct {
	target error
	key    string
}{
	// 认证（平台身份）
	{bizauth.ErrInvalidToken, "auth.invalid_credentials"},
	{bizauth.ErrPlatformUnavailable, "auth.platform_unavailable"},
	// 准入
	{bizadmission.ErrNotFound, "user.not_found"},
	{bizadmission.ErrDuplicatePlatformUser, "user.name_exists"},
	// 角色
	{role.ErrRoleNotFound, "role.not_found"},
	{role.ErrRoleHasUsers, "role.has_users"},
	{role.ErrBuiltinRoleLocked, "role.super_locked"},
	// 权限/菜单
	{bizperm.ErrPermissionNotFound, "permission.not_found"},
	{bizperm.ErrHasChildren, "permission.has_children"},
	{bizperm.ErrDuplicateCode, "permission.code_exists"},
	{bizperm.ErrDirTopLevelOnly, "permission.dir_top_level"},
	{bizperm.ErrMenuParentNotDir, "permission.menu_parent_dir"},
	{bizperm.ErrButtonParentNotMenu, "permission.button_parent_menu"},
	{bizperm.ErrParentIsSelf, "permission.parent_self"},
	{bizperm.ErrParentIsDescendant, "permission.parent_descendant"},
	// 文件
	{bizfile.ErrFileNotFound, "file.not_found"},
	{bizfile.ErrFileTypeDenied, "file.type_denied"},
	{bizfile.ErrPresignUnsupported, "file.presign_unsupported"},
	{bizfile.ErrDriverUnavailable, "file.driver_unavailable"},
	{bizfile.ErrFileGone, "file.gone"},
	// 异步导出
	{bizexport.ErrQueueFull, "export.queue_full"},
	{bizexport.ErrUnsupportedBiz, "export.unsupported_biz"},
	{bizexport.ErrNotFound, "export.not_found"},
	{bizexport.ErrNotOwner, "export.not_owner"},
	{bizexport.ErrNotReady, "export.not_ready"},
	// IP 黑名单
	{bizblacklist.ErrInvalidIP, "blacklist.invalid_ip"},
	{bizblacklist.ErrInvalidExpire, "blacklist.invalid_expire"},
	{bizblacklist.ErrIPExists, "blacklist.ip_exists"},
	{bizblacklist.ErrSelfBan, "blacklist.self_ban"},
	{bizblacklist.ErrNotFound, "blacklist.not_found"},
	// 租户
	{biztenant.ErrTenantNotFound, "tenant.not_found"},
	{biztenant.ErrDuplicateTenantName, "tenant.name_exists"},
	{biztenant.ErrDuplicateTenantCode, "tenant.code_exists"},
	{biztenant.ErrTenantInUse, "tenant.in_use"},
	// 应用用户（凭证/禁用语义与后台认证一致，复用其文案）
	{bizappuser.ErrAppUserNotFound, "appuser.not_found"},
	{bizappuser.ErrDuplicateUsername, "appuser.name_exists"},
	{bizappuser.ErrAppUserDisabled, "auth.account_disabled"},
	{bizappuser.ErrBadCredentials, "auth.invalid_credentials"},
	// 服务器监控
	{bizmonitor.ErrCollectFailed, "monitor.collect_failed"},
}

// init 将错误 -> i18n key 匹配函数注册到 response 包（response 不便反向依赖 server，走钩子）
func init() {
	response.ErrKeyFunc = func(err error) (string, bool) {
		for _, e := range errKeys {
			if errors.Is(err, e.target) {
				return e.key, true
			}
		}
		return "", false
	}
}
