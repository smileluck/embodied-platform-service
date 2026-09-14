package server

import (
	"errors"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	bizmerchant "github.com/smilex/smilex-admin-gin/internal/biz/merchant"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/biz/role"
	bizsession "github.com/smilex/smilex-admin-gin/internal/biz/session"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/internal/biz/user"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// errKeys 业务哨兵错误 -> 语言包 key 注册表（errors.Is 匹配，顺序即优先级）。
// 带参数的错误（file.too_large、blacklist.ip_banned）不在此列，由调用点带参渲染。
var errKeys = []struct {
	target error
	key    string
}{
	// 认证
	{bizauth.ErrInvalidCredentials, "auth.invalid_credentials"},
	{bizauth.ErrDisabledAccount, "auth.account_disabled"},
	{bizauth.ErrCaptcha, "auth.captcha_invalid"},
	// 用户
	{user.ErrUserNotFound, "user.not_found"},
	{user.ErrDuplicateUsername, "user.name_exists"},
	{user.ErrSuperAdminProtected, "user.super_protected"},
	{user.ErrDeleteSuperAdmin, "user.super_delete_forbidden"},
	// 角色
	{role.ErrRoleNotFound, "role.not_found"},
	{role.ErrRoleHasUsers, "role.has_users"},
	{role.ErrSuperRoleLocked, "role.super_locked"},
	// 权限/菜单
	{bizperm.ErrPermissionNotFound, "permission.not_found"},
	{bizperm.ErrHasChildren, "permission.has_children"},
	{bizperm.ErrDuplicateCode, "permission.code_exists"},
	{bizperm.ErrDirTopLevelOnly, "permission.dir_top_level"},
	{bizperm.ErrMenuParentNotDir, "permission.menu_parent_dir"},
	{bizperm.ErrButtonParentNotMenu, "permission.button_parent_menu"},
	{bizperm.ErrParentIsSelf, "permission.parent_self"},
	{bizperm.ErrParentIsDescendant, "permission.parent_descendant"},
	{bizperm.ErrWildcardLocked, "permission.wildcard_locked"},
	// 文件
	{bizfile.ErrFileNotFound, "file.not_found"},
	{bizfile.ErrFileTypeDenied, "file.type_denied"},
	{bizfile.ErrPresignUnsupported, "file.presign_unsupported"},
	{bizfile.ErrDriverUnavailable, "file.driver_unavailable"},
	// 异步导出
	{bizexport.ErrQueueFull, "export.queue_full"},
	{bizexport.ErrUnsupportedBiz, "export.unsupported_biz"},
	{bizexport.ErrNotFound, "export.not_found"},
	{bizexport.ErrNotOwner, "export.not_owner"},
	{bizexport.ErrNotReady, "export.not_ready"},
	// 会话
	{bizsession.ErrSessionNotFound, "session.not_found"},
	// IP 黑名单
	{bizblacklist.ErrInvalidIP, "blacklist.invalid_ip"},
	{bizblacklist.ErrInvalidExpire, "blacklist.invalid_expire"},
	{bizblacklist.ErrIPExists, "blacklist.ip_exists"},
	{bizblacklist.ErrSelfBan, "blacklist.self_ban"},
	{bizblacklist.ErrNotFound, "blacklist.not_found"},
	// 商户（开放 API 授权）
	{bizmerchant.ErrMerchantNotFound, "merchant.not_found"},
	{bizmerchant.ErrDuplicateCode, "merchant.code_exists"},
	{bizmerchant.ErrMerchantDisabled, "merchant.disabled"},
	{bizmerchant.ErrInvalidSign, "merchant.sign_invalid"},
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
