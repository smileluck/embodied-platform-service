package server

import (
	"errors"

	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	bizagent "github.com/smilex/smilex-admin-gin/internal/biz/agent"
	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	bizdict "github.com/smilex/smilex-admin-gin/internal/biz/dict"
	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	bizjob "github.com/smilex/smilex-admin-gin/internal/biz/job"
	bizmonitor "github.com/smilex/smilex-admin-gin/internal/biz/monitor"
	biznotice "github.com/smilex/smilex-admin-gin/internal/biz/notice"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/biz/role"
	bizsys "github.com/smilex/smilex-admin-gin/internal/biz/sysconfig"
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
	{role.ErrDuplicateName, "role.name_exists"},
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
	// 智能体（LLM 配置底座）
	{bizagent.ErrProviderNotFound, "agent.provider.not_found"},
	{bizagent.ErrProviderCodeExists, "agent.provider.code_exists"},
	{bizagent.ErrProviderHasModels, "agent.provider.has_models"},
	{bizagent.ErrProviderNoModels, "agent.provider.no_models"},
	{bizagent.ErrProviderDisabled, "agent.provider.disabled"},
	{bizagent.ErrModelNotFound, "agent.model.not_found"},
	{bizagent.ErrModelExists, "agent.model.exists"},
	{bizagent.ErrModelInUse, "agent.model.in_use"},
	{bizagent.ErrModelDisabled, "agent.model.disabled"},
	{bizagent.ErrAgentNotFound, "agent.not_found"},
	{bizagent.ErrAgentCodeExists, "agent.code_exists"},
	{bizagent.ErrAgentDisabled, "agent.disabled"},
	{bizagent.ErrDecryptFailed, "agent.decrypt_failed"},
	{bizagent.ErrLLMTimeout, "agent.timeout"},
	{bizagent.ErrLLMConnect, "agent.connect_failed"},
	{bizagent.ErrConversationNotFound, "agent.conversation.not_found"},
	{bizagent.ErrConversationAgentMismatch, "agent.conversation.agent_mismatch"},
	{bizagent.ErrUnknownTool, "agent.tool.unknown"},
	// 数据字典
	{bizdict.ErrTypeNotFound, "dict.type.not_found"},
	{bizdict.ErrTypeCodeExists, "dict.type.code_exists"},
	{bizdict.ErrTypeHasItems, "dict.type.has_items"},
	{bizdict.ErrItemNotFound, "dict.item.not_found"},
	{bizdict.ErrItemExists, "dict.item.exists"},
	// 系统参数
	{bizsys.ErrNotFound, "sysconfig.not_found"},
	{bizsys.ErrKeyExists, "sysconfig.key_exists"},
	{bizsys.ErrBadValue, "sysconfig.bad_value"},
	{bizsys.ErrKeyInvalid, "sysconfig.key_invalid"},
	// 通知公告
	{biznotice.ErrNotFound, "notice.not_found"},
	{biznotice.ErrInvalidTitle, "notice.inactive"},
	{biznotice.ErrInvalidTargets, "notice.invalid_targets"},
	{biznotice.ErrNotDelivered, "notice.not_delivered"},
	// 定时任务
	{bizjob.ErrNotFound, "job.not_found"},
	{bizjob.ErrNameExists, "job.name_exists"},
	{bizjob.ErrBadCron, "job.bad_cron"},
	{bizjob.ErrUnknownHandler, "job.unknown_handler"},
	{bizjob.ErrDisabled, "job.disabled"},
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
