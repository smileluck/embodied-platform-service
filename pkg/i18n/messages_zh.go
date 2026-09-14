package i18n

// messagesZh 中文文案表（默认语言，key 命名：模块.语义）
var messagesZh = map[string]string{
	// 认证
	"auth.invalid_credentials": "用户名或密码错误",
	"auth.account_disabled":    "账号已被禁用",
	"auth.captcha_invalid":     "验证码错误或已过期",
	// 用户
	"user.not_found":              "用户不存在",
	"user.name_exists":            "用户名已存在，请更换",
	"user.super_protected":        "无权操作超级管理员账号",
	"user.super_delete_forbidden": "超级管理员账号禁止删除",
	// 角色
	"role.not_found":    "角色不存在",
	"role.has_users":    "该角色下存在用户，请先移除用户与该角色的关联",
	"role.super_locked": "超级管理员角色为系统内置，禁止修改和操作",
	// 权限/菜单
	"permission.not_found":          "权限不存在",
	"permission.has_children":       "该节点下存在子级，请先删除子级节点",
	"permission.code_exists":        "权限编码已存在，请更换",
	"permission.dir_top_level":      "目录只能挂在顶级",
	"permission.menu_parent_dir":    "菜单的父级必须是目录",
	"permission.button_parent_menu": "权限点的父级必须是菜单",
	"permission.parent_self":        "父级不能是自身",
	"permission.parent_descendant":  "父级不能是自身的子级",
	"permission.wildcard_locked":    "超管通配权限禁止删除",
	// 文件
	"file.not_found":           "文件不存在",
	"file.too_large":           "文件大小超出限制（上限 %dMB）",
	"file.type_denied":         "该类型文件禁止上传",
	"file.no_file":             "请选择要上传的文件（表单字段 file）",
	"file.presign_unsupported": "该存储后端不支持预签名",
	"file.driver_unavailable":  "文件所属的存储后端未配置，无法访问",
	// 异步导出
	"export.queue_full":      "导出任务队列已满，请稍后重试",
	"export.unsupported_biz": "不支持的导出类型",
	"export.not_found":       "导出记录不存在",
	"export.not_owner":       "无权操作他人的导出记录",
	"export.not_ready":       "导出任务尚未完成，无法下载",
	// 会话/个人中心
	"session.not_found":          "会话不存在或已下线",
	"profile.wrong_old_password": "原密码不正确",
	// 通用/安全中间件
	"common.invalid_params":   "请求参数不合法",
	"security.invalid_chars":  "请求参数包含非法字符",
	"security.login_frequent": "登录尝试过于频繁，请稍后再试",
	"blacklist.ip_banned":     "该 IP 因连续登录失败已被临时封禁，请约 %d 分钟后再试",
	// IP 黑名单（管理员手工维护的持久化封禁）
	"blacklist.ip_blocked":     "该 IP 已被加入黑名单，访问被拒绝",
	"blacklist.invalid_ip":     "IP 地址格式不正确",
	"blacklist.invalid_expire": "过期时间不能早于当前时间",
	"blacklist.ip_exists":      "该 IP 已在黑名单中",
	"blacklist.self_ban":       "不能将当前使用的 IP 加入黑名单",
	"blacklist.not_found":      "黑名单记录不存在",
	// 商户（开放 API 授权）
	"merchant.not_found":    "商户不存在",
	"merchant.code_exists":  "商户编码已存在，请更换",
	"merchant.disabled":     "商户已被禁用",
	"merchant.sign_invalid": "签名校验失败",
	// 租户
	"tenant.not_found":   "租户不存在",
	"tenant.name_exists": "租户名称已存在，请更换",
	"tenant.code_exists": "租户编码已存在，请更换",
	"tenant.in_use":      "该租户下存在应用用户，请先移除关联",
	// 应用用户
	"appuser.not_found":   "应用用户不存在",
	"appuser.name_exists": "用户名已存在，请更换",
	// 开放 API 验签中间件
	"openapi.missing_sign_headers": "缺少签名请求头",
	"openapi.invalid_timestamp":    "时间戳无效或超出允许偏差",
	"openapi.invalid_nonce":        "随机串不合法",
	"openapi.nonce_replayed":       "请求重复（nonce 已被使用）",
	"openapi.service_unavailable":  "验签服务暂不可用，请稍后重试",
	// 菜单（key 为 menu. + 权限 code，未命中保留库中原名）
	"menu.menu:dashboard":    "首页",
	"menu.menu:system":       "系统管理",
	"menu.menu:user":         "用户管理",
	"menu.menu:role":         "角色管理",
	"menu.menu:menu":         "菜单管理",
	"menu.menu:online":       "在线用户",
	"menu.menu:about":        "关于我们",
	"menu.menu:log":          "日志管理",
	"menu.menu:loginLog":     "登录日志",
	"menu.menu:opLog":        "操作日志",
	"menu.menu:file":         "文件管理",
	"menu.menu:blacklist":    "IP黑名单",
	"menu.menu:openapi":      "开放API",
	"menu.menu:merchant":     "商户管理",
	"menu.menu:merchantLog":  "API调用日志",
	"menu.menu:tenantCenter": "租户中心",
	"menu.menu:tenant":       "租户管理",
	"menu.menu:appUser":      "应用用户",
}
