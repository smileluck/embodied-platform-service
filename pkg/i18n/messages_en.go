package i18n

// messagesEn 英文文案表（key 与中文表一一对应）
var messagesEn = map[string]string{
	// 认证
	"auth.invalid_credentials": "Invalid username or password",
	"auth.account_disabled":    "Account has been disabled",
	"auth.captcha_invalid":     "Captcha code is invalid or expired",
	// 用户
	"user.not_found":              "User not found",
	"user.name_exists":            "Username already exists, please choose another one",
	"user.super_protected":        "Not allowed to operate on the super admin account",
	"user.super_delete_forbidden": "The super admin account cannot be deleted",
	// 角色
	"role.not_found":    "Role not found",
	"role.has_users":    "Users are still assigned to this role; remove the associations first",
	"role.super_locked": "The super admin role is built-in and cannot be modified or deleted",
	// 权限/菜单
	"permission.not_found":          "Permission not found",
	"permission.has_children":       "Child nodes exist under this node; delete them first",
	"permission.code_exists":        "Permission code already exists, please choose another one",
	"permission.dir_top_level":      "A directory can only be placed at the top level",
	"permission.menu_parent_dir":    "A menu's parent must be a directory",
	"permission.button_parent_menu": "A button permission's parent must be a menu",
	"permission.parent_self":        "The parent cannot be the node itself",
	"permission.parent_descendant":  "The parent cannot be a descendant of the node itself",
	"permission.wildcard_locked":    "The super admin wildcard permission cannot be deleted",
	// 文件
	"file.not_found":           "File not found",
	"file.too_large":           "File size exceeds the limit (max %dMB)",
	"file.type_denied":         "This file type is not allowed to upload",
	"file.no_file":             "Please choose a file to upload (form field \"file\")",
	"file.presign_unsupported": "This storage backend does not support presigned URLs",
	"file.driver_unavailable":  "The storage backend for this file is not configured",
	// 异步导出
	"export.queue_full":      "The export queue is full, please try again later",
	"export.unsupported_biz": "Unsupported export type",
	"export.not_found":       "Export record not found",
	"export.not_owner":       "Not allowed to operate on other users' export records",
	"export.not_ready":       "The export task is not finished yet, download unavailable",
	// 会话/个人中心
	"session.not_found":          "Session does not exist or is already offline",
	"profile.wrong_old_password": "The old password is incorrect",
	// 通用/安全中间件
	"common.invalid_params":   "Invalid request parameters",
	"security.invalid_chars":  "Request parameters contain illegal characters",
	"security.login_frequent": "Too many login attempts, please try again later",
	"blacklist.ip_banned":     "This IP has been temporarily banned due to repeated login failures; please try again in about %d minutes",
	// IP blacklist (manually maintained, persistent)
	"blacklist.ip_blocked":     "This IP is on the blacklist; access denied",
	"blacklist.invalid_ip":     "Invalid IP address format",
	"blacklist.invalid_expire": "Expiration time cannot be earlier than now",
	"blacklist.ip_exists":      "This IP is already on the blacklist",
	"blacklist.self_ban":       "You cannot blacklist the IP you are currently using",
	"blacklist.not_found":      "Blacklist entry not found",
	// Merchant (Open API authorization)
	"merchant.not_found":    "Merchant not found",
	"merchant.code_exists":  "Merchant code already exists, please choose another one",
	"merchant.disabled":     "Merchant has been disabled",
	"merchant.sign_invalid": "Signature verification failed",
	// Tenant
	"tenant.not_found":   "Tenant not found",
	"tenant.name_exists": "Tenant name already exists, please choose another one",
	"tenant.code_exists": "Tenant code already exists, please choose another one",
	"tenant.in_use":      "App users are still assigned to this tenant; remove the associations first",
	// App user
	"appuser.not_found":   "App user not found",
	"appuser.name_exists": "Username already exists, please choose another one",
	// Open API sign middleware
	"openapi.missing_sign_headers": "Missing signature headers",
	"openapi.invalid_timestamp":    "Timestamp is invalid or out of the allowed skew",
	"openapi.invalid_nonce":        "Invalid nonce",
	"openapi.nonce_replayed":       "Duplicated request (nonce already used)",
	"openapi.service_unavailable":  "Signature service temporarily unavailable, please try again later",
	// 菜单（key 为 menu. + 权限 code，未命中保留库中原名）
	"menu.menu:dashboard":    "Dashboard",
	"menu.menu:system":       "System",
	"menu.menu:user":         "Users",
	"menu.menu:role":         "Roles",
	"menu.menu:menu":         "Menus",
	"menu.menu:online":       "Online Users",
	"menu.menu:about":        "About",
	"menu.menu:log":          "Logs",
	"menu.menu:loginLog":     "Login Logs",
	"menu.menu:opLog":        "Operation Logs",
	"menu.menu:file":         "Files",
	"menu.menu:blacklist":    "IP Blacklist",
	"menu.menu:openapi":      "Open API",
	"menu.menu:merchant":     "Merchants",
	"menu.menu:merchantLog":  "API Logs",
	"menu.menu:tenantCenter": "Tenant Center",
	"menu.menu:tenant":       "Tenants",
	"menu.menu:appUser":      "App Users",
}
