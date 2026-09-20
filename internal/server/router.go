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
	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	admissionsvc "github.com/smilex/smilex-admin-gin/internal/service/admission"
	agentsvc "github.com/smilex/smilex-admin-gin/internal/service/agent"
	appusersvc "github.com/smilex/smilex-admin-gin/internal/service/appuser"
	authsvc "github.com/smilex/smilex-admin-gin/internal/service/auth"
	blacklistsvc "github.com/smilex/smilex-admin-gin/internal/service/blacklist"
	devicesvc "github.com/smilex/smilex-admin-gin/internal/service/device"
	devmodelsvc "github.com/smilex/smilex-admin-gin/internal/service/devmodel"
	dictsvc "github.com/smilex/smilex-admin-gin/internal/service/dict"
	exportsvc "github.com/smilex/smilex-admin-gin/internal/service/export"
	filesvc "github.com/smilex/smilex-admin-gin/internal/service/file"
	logsvc "github.com/smilex/smilex-admin-gin/internal/service/log"
	monitorsvc "github.com/smilex/smilex-admin-gin/internal/service/monitor"
	permsvc "github.com/smilex/smilex-admin-gin/internal/service/permission"
	rolesvc "github.com/smilex/smilex-admin-gin/internal/service/role"
	syssvc "github.com/smilex/smilex-admin-gin/internal/service/sysconfig"
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
	cfg           *conf.Bootstrap
	auth          *authsvc.Service
	admission     *admissionsvc.Service
	admissionUC   *bizadmission.Usecase // PlatformAuth 中间件直连领域用例（首登引导/准入判定）
	role          *rolesvc.Service
	perm          *permsvc.Service
	log           *logsvc.Service
	file          *filesvc.Service
	export        *exportsvc.Service
	blacklist     *blacklistsvc.Service
	monitor       *monitorsvc.Service
	agent         *agentsvc.Service
	dict          *dictsvc.Service
	syscfg        *syssvc.Service
	tenant        *tenantsvc.Service
	appuser       *appusersvc.Service
	appuserUC     *bizappuser.Usecase    // AppJWT 中间件直连领域用例（校验用户启用状态）
	appIssuer     bizappuser.TokenIssuer // AppJWT 中间件解析 app-access token
	device        *devicesvc.Service
	devmodel      *devmodelsvc.Service
	rdb           *redis.Client // 通用限流（固定窗口计数）
	rbacCache     *cache.TwoLevel
	identityCache *cache.TwoLevel
	engine        *gin.Engine
	srv           *http.Server
}

// NewHTTPServer 构造并注册路由。
// rbacCache 为 wire 单例（与 admission/role 用例共享：准入/角色/权限变更时 Flush 即时生效）。
func NewHTTPServer(cfg *conf.Bootstrap, auth *authsvc.Service, admission *admissionsvc.Service,
	admissionUC *bizadmission.Usecase, role *rolesvc.Service, perm *permsvc.Service, log *logsvc.Service,
	file *filesvc.Service, export *exportsvc.Service, blacklist *blacklistsvc.Service,
	tenant *tenantsvc.Service, appuser *appusersvc.Service, appuserUC *bizappuser.Usecase,
	appIssuer bizappuser.TokenIssuer, device *devicesvc.Service, devmodel *devmodelsvc.Service,
	monitor *monitorsvc.Service, agent *agentsvc.Service, dict *dictsvc.Service, syscfg *syssvc.Service,
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

	s := &HTTPServer{
		cfg: cfg, auth: auth, admission: admission, admissionUC: admissionUC,
		role: role, perm: perm, log: log,
		file: file, export: export, blacklist: blacklist, tenant: tenant,
		appuser: appuser, appuserUC: appuserUC, appIssuer: appIssuer,
		device: device, devmodel: devmodel, monitor: monitor, agent: agent, dict: dict, syscfg: syscfg,
		rdb:       rdb,
		rbacCache: rbacCache.TwoLevel, identityCache: identityCache, engine: e,
	}
	s.registerRoutes()
	s.registerStatic()

	s.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: e,
	}
	return s
}

func (s *HTTPServer) registerRoutes() {
	// 持久化 IP 黑名单在认证之前拦截全部 /api/ 请求（静态前端资源不经过此组）
	v1 := s.engine.Group("/api/v1", middleware.IPBlacklist(s.blacklist.Checker()))

	// ---- 公开接口 ----
	// 登录限流（应用用户登录）：IP 固定窗口计数，沿用 bl:rl: 前缀，
	// 黑名单提前解封时联动清零；参数见 biz/blacklist 常量
	loginRateLimit := middleware.NewRateLimit(s.rdb, middleware.RateLimitConfig{
		KeyPrefix: "bl:rl:", Max: bizblacklist.LoginRateMax, Window: bizblacklist.LoginRateWindow, MessageKey: "security.login_frequent",
	})

	// ---- 应用用户认证（本地体系，与平台身份 typ 隔离；无验证码、无服务端会话） ----
	// 登录接口挂 IP 临时封禁 + 频率限制防护，防口令爆破
	appauthg := v1.Group("/app-auth")
	{
		appauthg.POST("/login", middleware.LoginIPGuard(s.blacklist.LoginGuard()), loginRateLimit, s.appLogin)
		appauthg.POST("/refresh", s.appRefresh)
	}

	// 应用用户自身数据接口：仅 AppJWT 认证（查库校验启用状态），不做 RBAC
	appAuth := v1.Group("/app-auth", middleware.AppJWT(s.appIssuer, s.appuserUC))
	{
		appAuth.GET("/profile", s.appProfile)
		appAuth.PUT("/password", s.appChangePassword)
	}

	// ---- 自身数据接口：仅平台认证（token 自省 + 本地准入），不做 RBAC ----
	// 管理端登录不在本服务（前端直调平台 /auth/login，token 双用）；
	// profile 是本人信息、menus 是已按角色过滤的本人菜单树，均无越权面；
	// 若纳入默认拒绝的 RBAC，仅绑定了菜单/按钮权限的普通用户登录后即 403 白屏。
	// OpLog 自动审计写请求（改资料/改密码）。
	basic := v1.Group("", middleware.PlatformAuth(s.auth, s.admissionUC, s.identityCache), middleware.OpLog(s.log))
	{
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

		basic.GET("/dicts/:code/items", s.listDictItemsByCode)
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

	appUsers := protected.Group("/app-users")
	{
		appUsers.GET("", s.listAppUsers)
		appUsers.POST("", s.createAppUser)
		appUsers.GET("/:id", s.getAppUser)
		appUsers.PUT("/:id", s.updateAppUser)
		appUsers.DELETE("/:id", s.deleteAppUser)
		// 重置密码（新密码由管理员指定，旧密码立即失效）
		appUsers.PUT("/:id/password", s.resetAppUserPassword)
	}

	// 服务器状态监控（只读快照；CPU%/网卡速率由后台采样器固定 3s 窗口差值计算）
	monitors := protected.Group("/monitor")
	{
		monitors.GET("", s.getServerStatus)
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

	{
		dictItems.PUT("/:id", s.updateDictItem)
		dictItems.DELETE("/:id", s.deleteDictItem)
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
	return s.srv.Shutdown(ctx)
}

// pageParams 解析分页参数：单页上限取运行时参数 page.sizeMax（系统参数页可调，未配置回退 100）
func (s *HTTPServer) pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > s.syscfg.IntDefault(c.Request.Context(), "page.sizeMax", 100) {
		size = 10
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
