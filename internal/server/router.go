package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	biztenantmember "github.com/smilex/smilex-admin-gin/internal/biz/tenantmember"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	admissionsvc "github.com/smilex/smilex-admin-gin/internal/service/admission"
	agentsvc "github.com/smilex/smilex-admin-gin/internal/service/agent"
	appusersvc "github.com/smilex/smilex-admin-gin/internal/service/appuser"
	authsvc "github.com/smilex/smilex-admin-gin/internal/service/auth"
	blacklistsvc "github.com/smilex/smilex-admin-gin/internal/service/blacklist"
	dashsvc "github.com/smilex/smilex-admin-gin/internal/service/dashboard"
	devicesvc "github.com/smilex/smilex-admin-gin/internal/service/device"
	devmodelsvc "github.com/smilex/smilex-admin-gin/internal/service/devmodel"
	dictsvc "github.com/smilex/smilex-admin-gin/internal/service/dict"
	exportsvc "github.com/smilex/smilex-admin-gin/internal/service/export"
	filesvc "github.com/smilex/smilex-admin-gin/internal/service/file"
	jobsvc "github.com/smilex/smilex-admin-gin/internal/service/job"
	logsvc "github.com/smilex/smilex-admin-gin/internal/service/log"
	mcpsvc "github.com/smilex/smilex-admin-gin/internal/service/mcp"
	monitorsvc "github.com/smilex/smilex-admin-gin/internal/service/monitor"
	noticesvc "github.com/smilex/smilex-admin-gin/internal/service/notice"
	notifysvc "github.com/smilex/smilex-admin-gin/internal/service/notify"
	permsvc "github.com/smilex/smilex-admin-gin/internal/service/permission"
	rolesvc "github.com/smilex/smilex-admin-gin/internal/service/role"
	skillsvc "github.com/smilex/smilex-admin-gin/internal/service/skill"
	syssvc "github.com/smilex/smilex-admin-gin/internal/service/sysconfig"
	tenantmembersvc "github.com/smilex/smilex-admin-gin/internal/service/tenantmember"
	tenantsvc "github.com/smilex/smilex-admin-gin/internal/service/tenant"
	"github.com/smilex/smilex-admin-gin/pkg/cache"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"go.uber.org/zap"
	"strconv"
)

// HTTPServer 聚合全部应用服务
type HTTPServer struct {
	cfg              *conf.Bootstrap
	auth             *authsvc.Service
	admission        *admissionsvc.Service
	admissionUC      *bizadmission.Usecase // PlatformAuth 中间件直连领域用例（首登引导/准入判定）
	role             *rolesvc.Service
	perm             *permsvc.Service
	log              *logsvc.Service
	file             *filesvc.Service
	export           *exportsvc.Service
	blacklist        *blacklistsvc.Service
	monitor          *monitorsvc.Service
	agent            *agentsvc.Service
	dict             *dictsvc.Service
	syscfg           *syssvc.Service
	notice           *noticesvc.Service
	job              *jobsvc.Service
	dashboard        *dashsvc.Service
	notify           *notifysvc.Service
	mcp              *mcpsvc.Service
	skill            *skillsvc.Service
	tenant           *tenantsvc.Service
	appuser          *appusersvc.Service
	tenantmember     *tenantmembersvc.Service // 租户端成员自助管理（/app-api/v1/members）
	tenantmemberUC   *biztenantmember.Usecase // 租户端角色查询（profile/TenantAdmin 中间件）
	appIds           bizauth.AppIdentitySource // AppAuth 中间件的平台应用用户身份源
	tenantUC         *biztenant.Usecase        // AppAuth 租户闸门（按平台租户 ID 只读定位本地租户）
	appIdentityCache *cache.TwoLevel           // App token 自省缓存（pat: 前缀）
	device           *devicesvc.Service
	devmodel         *devmodelsvc.Service
	rdb              *redis.Client // 通用限流（固定窗口计数）
	rbacCache        *cache.TwoLevel
	identityCache    *cache.TwoLevel
	engine           *gin.Engine
	srv              *http.Server
}

// NewHTTPServer 构造并注册路由。
// rbacCache 为 wire 单例（与 admission/role 用例共享：准入/角色/权限变更时 Flush 即时生效）。
func NewHTTPServer(cfg *conf.Bootstrap, auth *authsvc.Service, admission *admissionsvc.Service,
	admissionUC *bizadmission.Usecase, role *rolesvc.Service, perm *permsvc.Service, log *logsvc.Service,
	file *filesvc.Service, export *exportsvc.Service, blacklist *blacklistsvc.Service,
	tenant *tenantsvc.Service, appuser *appusersvc.Service,
	tenantmember *tenantmembersvc.Service, tenantmemberUC *biztenantmember.Usecase,
	appIds bizauth.AppIdentitySource, tenantUC *biztenant.Usecase,
	device *devicesvc.Service, devmodel *devmodelsvc.Service,
	monitor *monitorsvc.Service, agent *agentsvc.Service, dict *dictsvc.Service, syscfg *syssvc.Service,
	notice *noticesvc.Service, job *jobsvc.Service, dashboard *dashsvc.Service, notify *notifysvc.Service,
	mcp *mcpsvc.Service, skill *skillsvc.Service,
	rbacCache *data.RBACCache, rdb *redis.Client) *HTTPServer {
	gin.SetMode(cfg.Server.Mode)
	e := gin.New()
	// multipart 表单内存上限保持较小值（超出部分落临时文件）；上传大小由 handler 显式校验
	e.MaxMultipartMemory = 8 << 20
	e.Use(gin.Recovery(),
		middleware.I18n(),
		middleware.SecurityHeaders(),
		middleware.CORS(cfg.Server.CORSOrigins),
		middleware.XSSFilter(),
		middleware.SQLInjectionGuard(),
	)

	// 平台身份自省缓存（token 哈希 → 平台身份）：L1 30s + L2 60s，
	// 平台侧吊销/改密在 TTL 内感知（与平台统一账号决策一致）
	identityCache := cache.NewTwoLevel(rdb, "pid:", 30*time.Second, 60*time.Second, cfg.Cache.L2Enabled)
	// 应用用户（C 端）自省缓存独立前缀（app-access token 哈希 → App 主体），
	// 禁用生效延迟 = 平台查库即时 + 本缓存 TTL（30-60s），与 B 端同一权衡
	appIdentityCache := cache.NewTwoLevel(rdb, "pat:", 30*time.Second, 60*time.Second, cfg.Cache.L2Enabled)

	s := &HTTPServer{
		cfg: cfg, auth: auth, admission: admission, admissionUC: admissionUC,
		role: role, perm: perm, log: log,
		file: file, export: export, blacklist: blacklist, tenant: tenant,
		appuser: appuser,
		tenantmember: tenantmember, tenantmemberUC: tenantmemberUC,
		appIds:  appIds, tenantUC: tenantUC, appIdentityCache: appIdentityCache,
		device: device, devmodel: devmodel, monitor: monitor, agent: agent, dict: dict, syscfg: syscfg,
		notice: notice, job: job, dashboard: dashboard, notify: notify, mcp: mcp, skill: skill,
		rdb:     rdb,
		rbacCache: rbacCache.TwoLevel, identityCache: identityCache, engine: e,
	}
	s.registerRoutes()
	s.registerStatic()

	// 定时任务：播种内置任务并拉起 cron 调度器（租户投影对账/保留期清理等）。
	// 失败不阻断 HTTP 服务（任务体系是运营面），记错误日志可观测
	if err := job.EnsureSeededAndStart(); err != nil {
		logger.Error("job scheduler start failed", zap.Error(err))
	}

	s.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: e,
	}
	return s
}

func (s *HTTPServer) registerRoutes() {
	// 持久化 IP 黑名单在认证之前拦截全部 /api/ 请求（静态前端资源不经过此组）
	v1 := s.engine.Group("/api/v1", middleware.IPBlacklist(s.blacklist.Checker()))

	// ---- 认证代理（公开）：登录/刷新/验证码。登录经本服务代理平台公开 API，
	// 服务端落登录日志（含失败）；LoginIPGuard 临时封禁在限流之前生效（与 app 登录同策略），
	// 失败计数由中间件按响应状态码自动完成（401 计入、200 清零，handler 不重复调用）。
	v1.POST("/auth/login",
		middleware.LoginIPGuard(s.blacklist.LoginGuard()),
		middleware.NewRateLimit(s.rdb, middleware.RateLimitConfig{
			KeyPrefix: "bl:rl:", Max: 10, Window: time.Minute, MessageKey: "security.login_frequent",
		}),
		s.login)
	v1.POST("/auth/refresh",
		middleware.NewRateLimit(s.rdb, middleware.RateLimitConfig{
			KeyPrefix: "rl:auth-refresh:", Max: 60, Window: time.Minute, MessageKey: "security.rate_limited",
		}),
		s.refreshToken)
	v1.GET("/auth/captcha", s.captcha)

	// ---- App 面（/app-api/v1）：App（C 端）直调本系统的业务接口 ----
	// App 直连平台 /app-auth 登录（token 双用）；AppAuth 自省平台 app profile + 租户闸门
	// （X-Tenant-ID = 平台租户 ID，须∈用户归属集且本地已同步）；per-uid 限流。
	// 不挂 IPBlacklist/OpLog（管理端语义），业务端点后续按需求挂入本组。
	appAPI := s.engine.Group("/app-api/v1",
		middleware.AppAuth(s.appIds, s.tenantUC, s.appIdentityCache),
		middleware.NewRateLimit(s.rdb, middleware.RateLimitConfig{
			KeyPrefix: "rl:app-api:", Max: 120, Window: time.Minute,
			SubjectFunc: func(c *gin.Context) string {
				if sub := middleware.AppAuthSubject(c); sub != nil {
					return strconv.FormatUint(uint64(sub.UserID), 10)
				}
				return ""
			},
			MessageKey: "security.rate_limited",
		}))
	{
		appAPI.GET("/profile", s.appApiProfile)
		// 本人修改密码：持本人 token 代理平台（平台校验旧密码并吊销其他端会话）
		appAPI.PUT("/profile/password", s.appApiChangePassword)

		// ---- 租户端成员自助管理（仅 tenant_admin：本地角色绑定默认拒绝） ----
		// 移除成员=tenant_ids 差集更新（不删账号）；守卫（最后管理员/本人）在 biz 层
		members := appAPI.Group("/members", middleware.TenantAdmin(s.tenantmemberUC))
		{
			members.GET("", s.appApiListMembers)
			members.POST("", s.appApiCreateMember)
			members.PUT("/:id", s.appApiUpdateMember)
			members.PUT("/:id/status", s.appApiSetMemberStatus)
			members.PUT("/:id/password", s.appApiResetMemberPassword)
			members.PUT("/:id/role", s.appApiSetMemberRole)
			members.DELETE("/:id", s.appApiRemoveMember)
		}

		// ---- 租户端设备只读（归属即准入：成员可读，无指令下发） ----
		appAPI.GET("/devices", s.appApiListDevices)
		appAPI.GET("/devices/:id", s.appApiGetDevice)
		appAPI.GET("/devices/:id/shadow", s.appApiGetDeviceShadow)
		appAPI.GET("/devices/:id/telemetry", s.appApiGetDeviceTelemetry)
	}

	// ---- 自身数据接口：仅平台认证（token 自省 + 本地准入），不做 RBAC ----
	// 管理端登录由本服务公开路由代理平台（见上方 /auth/login）；
	// profile 是本人信息、menus 是已按角色过滤的本人菜单树，均无越权面；
	// 若纳入默认拒绝的 RBAC，仅绑定了菜单/按钮权限的普通用户登录后即 403 白屏。
	// OpLog 自动审计写请求（改资料/改密码/登出）。
	basic := v1.Group("", middleware.PlatformAuth(s.auth, s.admissionUC, s.identityCache), middleware.OpLog(s.log))
	{
		// 登出：带用户本人 token 代理平台吊销会话
		basic.POST("/auth/logout", s.logout)
		basic.GET("/auth/profile", func(c *gin.Context) {
			vo, err := s.auth.Profile(c.Request.Context(), middleware.Subject(c))
			if err != nil {
				response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
				return
			}
			response.OK(c, vo)
		})
		// 本人更新昵称/邮箱：以用户本人平台 token 代理到平台
		basic.PUT("/auth/profile", func(c *gin.Context) {
			var req authsvc.UpdateProfileRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
				return
			}
			if err := s.auth.UpdateProfile(c.Request.Context(), middleware.Token(c), req); err != nil {
				s.platformErr(c, err)
				return
			}
			response.OK(c, nil)
		})
		// 本人修改密码：代理到平台（平台侧校验旧密码并吊销其他端会话）
		basic.PUT("/auth/password", func(c *gin.Context) {
			var req authsvc.ChangePasswordRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
				return
			}
			if err := s.auth.ChangePassword(c.Request.Context(), middleware.Token(c), req); err != nil {
				s.platformErr(c, err)
				return
			}
			response.OK(c, nil)
		})
		// 当前用户可见菜单树
		basic.GET("/menus", func(c *gin.Context) {
			tree, err := s.perm.UserMenuTree(c.Request.Context(), middleware.Subject(c).UserID)
			if err != nil {
				response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
				return
			}
			response.OK(c, tree)
		})
		// 菜单搜索（顶栏命令面板）：当前用户可见菜单内按关键词模糊匹配
		basic.GET("/menus/search", func(c *gin.Context) {
			kw := strings.TrimSpace(c.Query("kw"))
			hits, err := s.perm.SearchUserMenus(c.Request.Context(), middleware.Subject(c).UserID, kw)
			if err != nil {
				response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
				return
			}
			if hits == nil {
				hits = []*bizperm.MenuHit{}
			}
			response.OK(c, hits)
		})

		basic.GET("/dashboard/stats", s.dashboardStats)
		basic.GET("/dicts/:code/items", s.listDictItemsByCode)
		basic.GET("/notices/active", s.listActiveNotices)
		basic.GET("/notices/unread-count", s.unreadNoticeCount)
		basic.POST("/notices/:id/read", s.readNotice)
		// 异步导出：记录归属当前用户，列表/下载/删除均强制按平台用户 ID 过滤（biz 层校验），
		// 与 profile/menus 同属自身数据接口，故仅认证不走 RBAC；导出入口（POST */export）在 protected 组按按钮权限点控制
		exports := basic.Group("/exports")
		{
			// recent=1 返回近期 5 条（任务浮层轮询）；否则分页返回本人记录
			exports.GET("", s.listExports)
			// 下载：302 到平台存储预签名 URL
			exports.GET("/:id/download", s.downloadExport)
			exports.DELETE("/:id", s.deleteExport)
		}
	}

	// ---- 受保护接口：平台认证 -> 操作日志（RBAC 拒绝的尝试也记录）-> RBAC ----
	authmw := middleware.PlatformAuth(s.auth, s.admissionUC, s.identityCache)
	protected := v1.Group("", authmw, middleware.OpLog(s.log), middleware.RBAC(s.auth, s.rbacCache))

	// 用户（成员管理：列表实时来自平台本商户绑定成员；新增/删除推送平台——
	// 新增=平台无此账号则创建有则绑定本商户，删除=仅解除关联账号本体保留；
	// 路由 :id 一律为平台用户 ID，本地准入开关/角色仍存投影）
	users := protected.Group("/users")
	{
		users.GET("", s.listUsers)
		users.POST("", s.addUser)
		users.GET("/:id", s.getUser)
		users.PUT("/:id/admission", s.setUserAdmission)
		users.PUT("/:id/roles", s.setUserRoles)
		users.DELETE("/:id", s.deleteUser)
		// 从平台拉取本商户绑定成员：补建缺失投影（成员=准入开启）/刷新快照
		users.POST("/sync", s.syncUsersFromPlatform)
		// 导出用户列表（查询条件透传 query，与列表页一致）
		users.POST("/export", func(c *gin.Context) { s.submitExport(c, "user") })
	}

	roles := protected.Group("/roles")
	{
		roles.GET("", s.listRoles)
		roles.POST("", s.createRole)
		roles.GET("/:id", s.getRole)
		roles.PUT("/:id", s.updateRole)
		roles.DELETE("/:id", s.deleteRole)
		roles.PUT("/:id/permissions", s.setRolePermissions)
	}

	perms := protected.Group("/permissions")
	{
		perms.GET("", s.listPerms)
		perms.POST("", s.createPerm)
		perms.GET("/:id", s.getPerm)
		perms.PUT("/:id", s.updatePerm)
		perms.DELETE("/:id", s.deletePerm)
	}

	opLogs := protected.Group("/operation-logs")
	{
		opLogs.GET("", s.listOperationLogs)
		opLogs.DELETE("", s.clearOperationLogs)
		opLogs.POST("/export", func(c *gin.Context) { s.submitExport(c, "op_log") })
	}

	loginLogs := protected.Group("/login-logs")
	{
		loginLogs.GET("", s.listLoginLogs)
		loginLogs.DELETE("", s.clearLoginLogs)
		loginLogs.POST("/export", func(c *gin.Context) { s.submitExport(c, "login_log") })
	}

	files := protected.Group("/files")
	{
		files.GET("", s.listFiles)
		files.POST("", s.uploadFile)
		// 下载/预览：302 到平台存储预签名 URL
		files.GET("/:id/raw", s.downloadFile)
		files.DELETE("/:id", s.deleteFile)
	}

	ipBlacklist := protected.Group("/ip-blacklist")
	{
		ipBlacklist.GET("", s.listIPBlacklist)
		ipBlacklist.POST("", s.createIPBlacklist)
		// 解封（软删留痕）
		ipBlacklist.DELETE("/:id", s.deleteIPBlacklist)
	}

	tenants := protected.Group("/tenants")
	{
		tenants.GET("", s.listTenants)
		tenants.POST("", s.createTenant)
		tenants.GET("/:id", s.getTenant)
		tenants.PUT("/:id", s.updateTenant)
		tenants.DELETE("/:id", s.deleteTenant)
		tenants.PUT("/:id/status", s.setTenantStatus)
		// 存量补链：把未同步的租户在平台建出/绑定并回填 platform_id
		tenants.POST("/:id/sync", s.syncTenant)
	}

	// 应用用户管理：2026-10-05 起经平台开放面实时消费（平台为唯一事实源；
	// tenant_ids/tenant_id 均为平台租户 ID；开放面无单查端点，编辑用列表行数据）
		appUsers := protected.Group("/app-users")
		{
			appUsers.GET("", s.listAppUsers)
			appUsers.POST("", s.createAppUser)
			appUsers.PUT("/:id", s.updateAppUser)
			appUsers.DELETE("/:id", s.deleteAppUser)
			// 重置密码（新密码由管理员指定，旧密码立即失效）
			appUsers.PUT("/:id/password", s.resetAppUserPassword)
			// 租户端角色（管理面设置/修复租户管理员；目标须已是该租户成员）
			appUsers.PUT("/:id/tenant-role", s.setAppUserTenantRole)
		}

	// 服务器状态监控（只读快照；CPU%/网卡速率由后台采样器固定 3s 窗口差值计算）
	monitors := protected.Group("/monitor")
	{
		monitors.GET("", s.getServerStatus)
		// 历史快照回看（menu:monitor 复用查看权限点）
		monitors.GET("/history", s.getMonitorHistory)
	}

	// 智能体（LLM 配置底座）：供应商 -> 模型 -> Agent
	agentProviders := protected.Group("/agent/providers")
	{
		agentProviders.GET("", s.listAgentProviders)
		agentProviders.POST("", s.createAgentProvider)
		agentProviders.GET("/:id", s.getAgentProvider)
		agentProviders.PUT("/:id", s.updateAgentProvider)
		agentProviders.DELETE("/:id", s.deleteAgentProvider)
		// 连通性测试：真实调用一次上游（model_id 缺省取首个启用模型）
		agentProviders.POST("/:id/test", s.testAgentProvider)
		// 拉取上游 /models 列表（录入辅助）
		agentProviders.GET("/:id/remote-models", s.listAgentRemoteModels)
	}

	agentModels := protected.Group("/agent/models")
	{
		agentModels.GET("", s.listAgentModels)
		agentModels.POST("", s.createAgentModel)
		agentModels.PUT("/:id", s.updateAgentModel)
		agentModels.DELETE("/:id", s.deleteAgentModel)
		agentModels.POST("/:id/test", s.testAgentModel)
	}

	// ---- MCP 服务（智能体工具源）：配置 CRUD + 连通测试 ----
	mcpServers := protected.Group("/mcp/servers")
	{
		mcpServers.GET("", s.listMcpServers)
		mcpServers.POST("", s.createMcpServer)
		mcpServers.GET("/:id", s.getMcpServer)
		mcpServers.PUT("/:id", s.updateMcpServer)
		mcpServers.DELETE("/:id", s.deleteMcpServer)
		// 连通测试：真实握手远端（外呼请求，按用户限流防滥用）
		mcpServers.POST("/:id/test", middleware.NewRateLimit(s.rdb, middleware.RateLimitConfig{
			KeyPrefix: "rl:mcp-test:", Max: 10, Window: time.Minute, ByUser: true, MessageKey: "security.rate_limited",
		}), s.testMcpServer)
	}

	// ---- 技能管理（多文件技能包：附属文件随技能整体提交） ----
	skills := protected.Group("/skills")
	{
		skills.GET("", s.listSkills)
		skills.POST("", s.createSkill)
		skills.GET("/:id", s.getSkill)
		skills.PUT("/:id", s.updateSkill)
		skills.DELETE("/:id", s.deleteSkill)
	}

	// ---- 定时任务 ----
	jobs := protected.Group("/jobs")
	{
		jobs.GET("", s.listJobs)
		jobs.GET("/handlers", s.listJobHandlers)
		jobs.POST("", s.createJob)
		jobs.GET("/:id", s.getJob)
		jobs.PUT("/:id", s.updateJob)
		jobs.PUT("/:id/status", s.setJobStatus)
		jobs.DELETE("/:id", s.deleteJob)
		jobs.POST("/:id/run", s.runJobOnce)
		jobs.GET("/:id/logs", s.listJobLogs)
	}

	// ---- 通知公告（管理端） ----
	notices := protected.Group("/notices")
	{
		notices.GET("", s.listNotices)
		notices.POST("", s.createNotice)
		notices.GET("/:id", s.getNotice)
		notices.PUT("/:id", s.updateNotice)
		notices.DELETE("/:id", s.deleteNotice)
		// 发布表单送达范围选项（公告权限即可，无需角色/用户管理权限）
		notices.GET("/options/roles", s.noticeRoleOptions)
		notices.GET("/options/users", s.noticeUserOptions)
	}

	// ---- 告警通知 ----
	notifyChannels := protected.Group("/notify/channels")
	{
		notifyChannels.GET("", s.listNotifyChannels)
		notifyChannels.POST("", s.createNotifyChannel)
		notifyChannels.GET("/:id", s.getNotifyChannel)
		notifyChannels.PUT("/:id", s.updateNotifyChannel)
		notifyChannels.DELETE("/:id", s.deleteNotifyChannel)
		notifyChannels.POST("/:id/test", s.testNotifyChannel)
	}
	notifyRules := protected.Group("/notify/rules")
	{
		notifyRules.GET("", s.listNotifyRules)
		notifyRules.POST("", s.createNotifyRule)
		notifyRules.GET("/:id", s.getNotifyRule)
		notifyRules.PUT("/:id", s.updateNotifyRule)
		notifyRules.DELETE("/:id", s.deleteNotifyRule)
	}
	notifyRecords := protected.Group("/notify/records")
	{
		notifyRecords.GET("", s.listNotifyRecords)
		notifyRecords.DELETE("", s.clearNotifyRecords)
	}

	// ---- 系统参数（运行时可调） ----
	syscfgs := protected.Group("/sys-configs")
	{
		syscfgs.GET("", s.listSysConfigs)
		syscfgs.POST("", s.createSysConfig)
		syscfgs.PUT("/:key", s.updateSysConfig)
		syscfgs.DELETE("/:key", s.deleteSysConfig)
	}

	// ---- 数据字典 ----
	dictTypes := protected.Group("/dict-types")
	{
		dictTypes.GET("", s.listDictTypes)
		dictTypes.POST("", s.createDictType)
		dictTypes.GET("/:id", s.getDictType)
		dictTypes.PUT("/:id", s.updateDictType)
		dictTypes.DELETE("/:id", s.deleteDictType)
		dictTypes.GET("/:id/items", s.listDictItems)
		dictTypes.POST("/:id/items", s.createDictItem)
	}
	dictItems := protected.Group("/dict-items")
	{
		dictItems.PUT("/:id", s.updateDictItem)
		dictItems.DELETE("/:id", s.deleteDictItem)
	}

	// 可绑定工具清单（Agent 表单多选）
	agentTools := protected.Group("/agent/tools")
	{
		agentTools.GET("", s.listAgentTools)
	}

	// 用量统计（读聚合）
	usage := protected.Group("/agent/usage")
	{
		usage.GET("", s.getAgentUsage)
	}

	// 会话（本人数据，biz 层强制 user_id 过滤）
	convs := protected.Group("/agent/conversations")
	{
		convs.GET("", s.listAgentConversations)
		convs.POST("", s.createAgentConversation)
		convs.GET("/:id/messages", s.listAgentConversationMessages)
		convs.PUT("/:id", s.renameAgentConversation)
		convs.DELETE("/:id", s.deleteAgentConversation)
	}

	agents := protected.Group("/agents")
	{
		agents.GET("", s.listAgents)
		agents.POST("", s.createAgent)
		agents.GET("/:id", s.getAgent)
		agents.PUT("/:id", s.updateAgent)
		agents.DELETE("/:id", s.deleteAgent)
		// 调试对话（Playground，SSE 流式；无状态，不落库）；
		// 按用户限流：每次调用都产生真实上游 token 费用，防误用/滥用刷量
		agents.POST("/:id/chat", middleware.NewRateLimit(s.rdb, middleware.RateLimitConfig{
			KeyPrefix: "rl:agent-chat:", Max: 20, Window: time.Minute, ByUser: true, MessageKey: "security.rate_limited",
		}), s.chatAgent)
	}

	// 设备（纯代理平台开放面；租户范围由平台按商户绑定服务端收敛）
	devices := protected.Group("/devices")
	{
		devices.GET("", s.listDevices)
		devices.POST("", s.registerDevice)
		devices.GET("/:id", s.getDevice)
		devices.GET("/:id/shadow", s.getDeviceShadow)
		devices.GET("/:id/commands", s.listDeviceCommands)
		devices.POST("/:id/commands", s.issueDeviceCommand)
		devices.GET("/:id/telemetry", s.getDeviceTelemetry)
		devices.GET("/:id/data-events", s.listDeviceDataEvents)
		protected.GET("/device-commands/:id", s.getDeviceCommand)
	}

	// 设备型号（纯代理平台管理面；创建须绑物模型节点+已发布版本）
	models := protected.Group("/device-models")
	{
		models.GET("", s.listDeviceModels)
		models.POST("", s.createDeviceModel)
		models.GET("/:id", s.getDeviceModel)
		models.PUT("/:id", s.updateDeviceModel)
		models.DELETE("/:id", s.deleteDeviceModel)
	}

	// 物模型只读选择器（型号创建表单数据源；物模型管理列入后续规划）
	tms := protected.Group("/thing-models")
	{
		tms.GET("", s.listThingModelNodes)
		tms.GET("/:id/versions", s.listThingModelVersions)
	}
}

// Start 启动 HTTP 服务（阻塞）
func (s *HTTPServer) Start() error {
	return s.srv.ListenAndServe()
}

// registerStatic 托管前端 SPA 产物（web/dist），存在时启用；SPA history 路由 fallback 到 index.html
func (s *HTTPServer) registerStatic() {
	dir := s.cfg.Server.StaticDir
	if dir == "" {
		dir = "web/dist"
	}
	if index, err := filepath.Abs(filepath.Join(dir, "index.html")); err == nil {
		if _, err := os.Stat(index); err == nil {
			// assets 文件名带 hash，可长期缓存；index.html 禁止缓存避免发版后白屏
			s.engine.Static("/assets", filepath.Join(dir, "assets"))
			s.engine.NoRoute(func(c *gin.Context) {
				if strings.HasPrefix(c.Request.URL.Path, "/api/") {
					response.NotFound(c, "not found")
					return
				}
				c.Header("Cache-Control", "no-cache")
				c.File(index)
			})
			logger.Info("serving static frontend", zap.String("dir", dir))
		}
	}
}

// Stop 优雅关停
func (s *HTTPServer) Stop(ctx context.Context) error {
	s.job.Stop() // 先停 cron 调度器，再关 HTTP（避免停机窗口内新任务触发）
	return s.srv.Shutdown(ctx)
}

// pageParams 解析分页参数：单页上限取运行时参数 page.sizeMax（系统参数页可调，未配置回退 100）
func (s *HTTPServer) pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	// 未传或负数回落默认 10；显式传 0 = 全量（引用数据整表拉取，仓储层识别）
	if size < 0 || (size == 0 && c.Query("page_size") == "") {
		size = 10
	}
	max := s.syscfg.IntDefault(c.Request.Context(), "page.sizeMax", 100)
	if size > max {
		size = max // 运行时上限夹取（page.sizeMax 可调）
	}
	return page, size
}

func idParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "invalid id")
		return 0, false
	}
	return uint(id), true
}

// truncate 按字节长度截断（UA 摘要存储用）
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
