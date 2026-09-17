// Package server 传输层：Gin HTTP Server 与路由注册。
// 切换 Kratos 时本层是唯一需要替换的层（由 proto 生成的 HTTP/gRPC server 代替）。
package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/biz/role"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	admissionsvc "github.com/smilex/smilex-admin-gin/internal/service/admission"
	appusersvc "github.com/smilex/smilex-admin-gin/internal/service/appuser"
	authsvc "github.com/smilex/smilex-admin-gin/internal/service/auth"
	blacklistsvc "github.com/smilex/smilex-admin-gin/internal/service/blacklist"
	devicesvc "github.com/smilex/smilex-admin-gin/internal/service/device"
	devmodelsvc "github.com/smilex/smilex-admin-gin/internal/service/devmodel"
	exportsvc "github.com/smilex/smilex-admin-gin/internal/service/export"
	filesvc "github.com/smilex/smilex-admin-gin/internal/service/file"
	logsvc "github.com/smilex/smilex-admin-gin/internal/service/log"
	permsvc "github.com/smilex/smilex-admin-gin/internal/service/permission"
	rolesvc "github.com/smilex/smilex-admin-gin/internal/service/role"
	tenantsvc "github.com/smilex/smilex-admin-gin/internal/service/tenant"
	"github.com/smilex/smilex-admin-gin/pkg/cache"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"go.uber.org/zap"
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
	tenant        *tenantsvc.Service
	appuser       *appusersvc.Service
	appuserUC     *bizappuser.Usecase    // AppJWT 中间件直连领域用例（校验用户启用状态）
	appIssuer     bizappuser.TokenIssuer // AppJWT 中间件解析 app-access token
	device        *devicesvc.Service
	devmodel      *devmodelsvc.Service
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
	rbacCache *data.RBACCache, rdb *redis.Client) *HTTPServer {
	gin.SetMode(cfg.Server.Mode)
	e := gin.New()
	// multipart 表单内存上限保持较小值（超出部分落临时文件）；上传大小由 handler 显式校验
	e.MaxMultipartMemory = 8 << 20
	e.Use(gin.Recovery(),
		middleware.I18n(),
		middleware.SecurityHeaders(),
		middleware.CORS(),
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
		device: device, devmodel: devmodel,
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

	// ---- 应用用户认证（本地体系，与平台身份 typ 隔离；无验证码、无服务端会话） ----
	// 登录接口挂 IP 临时封禁 + 频率限制防护，防口令爆破
	appauthg := v1.Group("/app-auth")
	{
		appauthg.POST("/login", middleware.LoginIPGuard(s.blacklist.LoginGuard()), middleware.LoginRateLimit(s.blacklist.LoginGuard()), s.appLogin)
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

// ---- 用户（准入管理） ----

type listResult struct {
	List interface{} `json:"list"`
	Page interface{} `json:"page"`
}

func (s *HTTPServer) listUsers(c *gin.Context) {
	page, size := pageParams(c)
	var req admissionsvc.ListRequest
	_ = c.ShouldBindQuery(&req)
	users, pg, err := s.admission.List(c.Request.Context(), req, page, size)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, listResult{List: users, Page: pg})
}

func (s *HTTPServer) getUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.admission.Get(c.Request.Context(), id)
	if err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, vo)
}

// addUser 新增成员：推送平台（无此账号则创建并绑定本商户，有则仅绑定）并建本地准入投影
func (s *HTTPServer) addUser(c *gin.Context) {
	var req admissionsvc.AddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	p, existed, err := s.admission.Add(c.Request.Context(), req)
	if err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, gin.H{"projection": p, "existed": existed})
}

func (s *HTTPServer) setUserAdmission(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req admissionsvc.SetEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.admission.SetEnabled(c.Request.Context(), id, *req.Enabled); err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setUserRoles(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req admissionsvc.SetRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.admission.SetRoles(c.Request.Context(), id, req.RoleIDs); err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, nil)
}

// deleteUser 移除成员：解除平台侧绑定（账号本体保留）并删除本地投影
func (s *HTTPServer) deleteUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.admission.Delete(c.Request.Context(), id); err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) syncUsersFromPlatform(c *gin.Context) {
	created, refreshed, err := s.admission.SyncFromPlatform(c.Request.Context())
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, gin.H{"created": created, "refreshed": refreshed})
}

// admissionErr 准入/成员操作错误映射：不存在 404；平台信封错误透传；其余 400
func (s *HTTPServer) admissionErr(c *gin.Context, err error) {
	if isErr(err, bizadmission.ErrNotFound) {
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
		return
	}
	var perr *platformError
	if errorsAs(err, &perr) {
		s.platformErr(c, err)
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

// ---- 角色 ----

func (s *HTTPServer) listRoles(c *gin.Context) {
	page, size := pageParams(c)
	roles, pg, err := s.role.List(c.Request.Context(), role.Query{Name: c.Query("name")}, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: roles, Page: pg})
}

func (s *HTTPServer) createRole(c *gin.Context) {
	var req rolesvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	r, err := s.role.Create(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, r)
}

func (s *HTTPServer) getRole(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	r, err := s.role.Get(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, r)
}

func (s *HTTPServer) updateRole(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req rolesvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.role.Update(c.Request.Context(), id, req); err != nil {
		s.roleErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteRole(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.role.Delete(c.Request.Context(), id); err != nil {
		s.roleErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setRolePermissions(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req rolesvc.SetPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.role.SetPermissions(c.Request.Context(), id, req); err != nil {
		s.roleErr(c, err)
		return
	}
	response.OK(c, nil)
}

// roleErr 角色操作错误映射：超管角色保护类返回 403，其余返回 400
func (s *HTTPServer) roleErr(c *gin.Context, err error) {
	if isErr(err, role.ErrSuperRoleLocked) {
		response.FailI18n(c, http.StatusForbidden, response.CodeForbidden, err)
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

// ---- 权限 ----

func (s *HTTPServer) listPerms(c *gin.Context) {
	page, size := pageParams(c)
	// page_size=0：全量返回（菜单管理树/角色分配权限树需要整表构建，分页会静默截断）
	if c.Query("page_size") == "0" {
		size = 0
	}
	q := bizperm.Query{Type: c.Query("type")}
	ps, pg, err := s.perm.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: ps, Page: pg})
}

func (s *HTTPServer) createPerm(c *gin.Context) {
	var req permsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	p, err := s.perm.Create(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, p)
}

func (s *HTTPServer) getPerm(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	p, err := s.perm.Get(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, p)
}

func (s *HTTPServer) updatePerm(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req permsvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.perm.Update(c.Request.Context(), id, req); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deletePerm(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.perm.Delete(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

// ---- 日志 ----

// logListResult 日志列表响应（附保留天数，前端展示保留说明用）
type logListResult struct {
	List          interface{} `json:"list"`
	Page          interface{} `json:"page"`
	RetentionDays int         `json:"retention_days"`
}

func (s *HTTPServer) listOperationLogs(c *gin.Context) {
	page, size := pageParams(c)
	q := bizlog.OperationLogQuery{Username: c.Query("username"), Method: c.Query("method"), Keyword: c.Query("kw")}
	if t, ok := parseUnixParam(c.Query("start")); ok {
		q.Start = t
	}
	if t, ok := parseUnixParam(c.Query("end")); ok {
		q.End = t
	}
	logs, pg, err := s.log.ListOperationLogs(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, logListResult{List: logs, Page: pg, RetentionDays: s.log.RetentionDays()})
}

func (s *HTTPServer) clearOperationLogs(c *gin.Context) {
	n, err := s.log.ClearOperationLogs(c.Request.Context())
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, gin.H{"deleted": n})
}

// ---- 文件 ----

func (s *HTTPServer) listFiles(c *gin.Context) {
	page, size := pageParams(c)
	files, pg, err := s.file.List(c.Request.Context(), bizfile.Query{Name: c.Query("name")}, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: files, Page: pg})
}

func (s *HTTPServer) uploadFile(c *gin.Context) {
	sub := middleware.Subject(c)
	fh, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "file.no_file"))
		return
	}
	if max := s.cfg.Storage.MaxSizeMB << 20; max > 0 && fh.Size > max {
		response.BadRequest(c, i18n.T(c.Request.Context(), "file.too_large", s.cfg.Storage.MaxSizeMB))
		return
	}
	src, err := fh.Open()
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	defer src.Close()
	vo, err := s.file.Upload(c.Request.Context(), fh.Filename, src, fh.Size, sub.UserID, sub.Username)
	if err != nil {
		s.fileErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) downloadFile(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	d, err := s.file.ResolveDownload(c.Request.Context(), id)
	if err != nil {
		s.fileErr(c, err)
		return
	}
	// 平台存储：鉴权通过后 302 到短时效预签名 URL（云后端原生预签名 / local 后端网关代理 URL）
	if d.URL != "" {
		c.Redirect(http.StatusFound, d.URL)
		return
	}
	// 历史本地驱动：后端代理流式输出
	defer d.Body.Close()
	disposition := "attachment"
	if d.Inline && c.Query("download") == "" {
		disposition = "inline"
	}
	c.Header("Content-Type", d.ContentType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", contentDisposition(disposition, d.File.Name))
	if d.File.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(d.File.Size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, d.Body)
}

func (s *HTTPServer) deleteFile(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.file.Delete(c.Request.Context(), id); err != nil {
		s.fileErr(c, err)
		return
	}
	response.OK(c, nil)
}

// fileErr 文件操作错误映射：不存在 404，入参类 400，存储后端未配置 503，其余 500
func (s *HTTPServer) fileErr(c *gin.Context, err error) {
	switch {
	case isErr(err, bizfile.ErrFileNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, bizfile.ErrFileTooLarge):
		// 上限参数按当前配置渲染（biz 层只返回哨兵错误）
		response.BadRequest(c, i18n.T(c.Request.Context(), "file.too_large", s.cfg.Storage.MaxSizeMB))
	case isErr(err, bizfile.ErrFileTypeDenied):
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	case isErr(err, bizfile.ErrDriverUnavailable):
		response.FailI18n(c, http.StatusServiceUnavailable, response.CodeErr, err)
	default:
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
	}
}

// ---- IP 黑名单 ----

func (s *HTTPServer) listIPBlacklist(c *gin.Context) {
	page, size := pageParams(c)
	list, pg, err := s.blacklist.List(c.Request.Context(), bizblacklist.Query{IP: c.Query("ip")}, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createIPBlacklist(c *gin.Context) {
	sub := middleware.Subject(c)
	var req blacklistsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	// 防呆：禁止封禁当前操作者自己的 IP（归一化后比较，避免自我锁定）
	if ip := net.ParseIP(strings.TrimSpace(req.IP)); ip != nil && ip.String() == c.ClientIP() {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, bizblacklist.ErrSelfBan)
		return
	}
	vo, err := s.blacklist.Create(c.Request.Context(), req, sub.UserID, sub.Username)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) deleteIPBlacklist(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.blacklist.Delete(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

// ---- 租户 ----

func (s *HTTPServer) listTenants(c *gin.Context) {
	page, size := pageParams(c)
	q := biztenant.Query{
		Name: strings.TrimSpace(c.Query("name")),
		Code: strings.TrimSpace(c.Query("code")),
	}
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	list, pg, err := s.tenant.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createTenant(c *gin.Context) {
	var req tenantsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.tenant.Create(c.Request.Context(), req)
	if err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) getTenant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.tenant.Get(c.Request.Context(), id)
	if err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) updateTenant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantsvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenant.Update(c.Request.Context(), id, req); err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteTenant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.tenant.Delete(c.Request.Context(), id); err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setTenantStatus(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantsvc.SetStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenant.SetStatus(c.Request.Context(), id, req); err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, nil)
}

// syncTenant 存量补链：未同步租户在平台创建/绑定并回填 platform_id
func (s *HTTPServer) syncTenant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.tenant.SyncExisting(c.Request.Context(), id)
	if err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, vo)
}

// tenantErr 租户操作错误映射：不存在 404，关联应用用户/平台冲突 409，平台调用失败按平台状态透传
func (s *HTTPServer) tenantErr(c *gin.Context, err error) {
	switch {
	case isErr(err, biztenant.ErrTenantNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, biztenant.ErrTenantInUse):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	default:
		var perr *platformError
		if errorsAs(err, &perr) {
			s.platformErr(c, err)
			return
		}
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	}
}

// ---- 应用用户（管理端） ----

func (s *HTTPServer) listAppUsers(c *gin.Context) {
	page, size := pageParams(c)
	q := bizappuser.Query{
		Keyword: strings.TrimSpace(c.Query("kw")),
		Phone:   strings.TrimSpace(c.Query("phone")),
	}
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	if v := c.Query("tenant_id"); v != "" {
		if tid, err := strconv.ParseUint(v, 10, 64); err == nil && tid > 0 {
			t := uint(tid)
			q.TenantID = &t
		}
	}
	list, pg, err := s.appuser.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createAppUser(c *gin.Context) {
	var req appusersvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.appuser.Create(c.Request.Context(), req)
	if err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) getAppUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.appuser.Get(c.Request.Context(), id)
	if err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) updateAppUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req appusersvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.appuser.Update(c.Request.Context(), id, req); err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteAppUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.appuser.Delete(c.Request.Context(), id); err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) resetAppUserPassword(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req appusersvc.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.appuser.ResetPassword(c.Request.Context(), id, req); err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, nil)
}

// appuserErr 应用用户操作错误映射：不存在 404，其余 400
func (s *HTTPServer) appuserErr(c *gin.Context, err error) {
	if isErr(err, bizappuser.ErrAppUserNotFound) {
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

// ---- 应用用户独立认证（app-auth） ----

// appLogin 应用用户登录：成功返回令牌对 + 用户信息；失败统一 401（防爆破计数由 LoginIPGuard 负责）
func (s *HTTPServer) appLogin(c *gin.Context) {
	var req appusersvc.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.appuser.Login(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusUnauthorized, response.CodeUnauthorized, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) appRefresh(c *gin.Context) {
	var req appusersvc.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	tp, err := s.appuser.Refresh(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusUnauthorized, response.CodeUnauthorized, err)
		return
	}
	response.OK(c, tp)
}

func (s *HTTPServer) appProfile(c *gin.Context) {
	sub := middleware.AppSubject(c)
	vo, err := s.appuser.Profile(c.Request.Context(), sub.UserID)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, vo)
}

// appChangePassword 本人修改密码（校验旧密码）
func (s *HTTPServer) appChangePassword(c *gin.Context) {
	sub := middleware.AppSubject(c)
	var req appusersvc.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.appuser.ChangePassword(c.Request.Context(), sub.Username, req); err != nil {
		if isErr(err, bizappuser.ErrBadCredentials) {
			response.BadRequest(c, i18n.T(c.Request.Context(), "profile.wrong_old_password"))
			return
		}
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

// ---- 异步导出 ----

// submitExport 提交导出任务：原始查询条件（query，剔除分页参数）快照进记录，
// worker 按同一套条件分批拉数，保证导出结果与列表页所见一致
func (s *HTTPServer) submitExport(c *gin.Context, biz string) {
	sub := middleware.Subject(c)
	params := c.Request.URL.Query()
	params.Del("page")
	params.Del("page_size")
	vo, err := s.export.Submit(c.Request.Context(), biz, params, sub.UserID, sub.Username)
	if err != nil {
		switch {
		case isErr(err, bizexport.ErrQueueFull):
			response.FailI18n(c, http.StatusTooManyRequests, response.CodeErr, err)
		case isErr(err, bizexport.ErrUnsupportedBiz):
			response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		default:
			response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		}
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) listExports(c *gin.Context) {
	sub := middleware.Subject(c)
	if c.Query("recent") == "1" {
		vos, err := s.export.Recent(c.Request.Context(), sub.UserID, 5)
		if err != nil {
			response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
			return
		}
		response.OK(c, vos)
		return
	}
	page, size := pageParams(c)
	vos, pg, err := s.export.List(c.Request.Context(), sub.UserID, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: vos, Page: pg})
}

func (s *HTTPServer) downloadExport(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sub := middleware.Subject(c)
	d, err := s.export.ResolveDownload(c.Request.Context(), id, sub.UserID)
	if err != nil {
		s.exportErr(c, err)
		return
	}
	// 平台存储：鉴权通过后 302 到短时效预签名 URL
	if d.URL != "" {
		c.Redirect(http.StatusFound, d.URL)
		return
	}
	// 历史本地驱动：后端代理流式输出（强制 attachment + nosniff，CSV 不内联渲染）
	defer d.Body.Close()
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", contentDisposition("attachment", d.Record.Name))
	if d.Record.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(d.Record.Size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, d.Body)
}

func (s *HTTPServer) deleteExport(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sub := middleware.Subject(c)
	if err := s.export.Delete(c.Request.Context(), id, sub.UserID); err != nil {
		s.exportErr(c, err)
		return
	}
	response.OK(c, nil)
}

// exportErr 导出操作错误映射：不存在 404，越权 403，未完成 409，队列满 429，存储后端未配置 503，其余 500
func (s *HTTPServer) exportErr(c *gin.Context, err error) {
	switch {
	case isErr(err, bizexport.ErrNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, bizexport.ErrNotOwner):
		response.FailI18n(c, http.StatusForbidden, response.CodeForbidden, err)
	case isErr(err, bizexport.ErrNotReady):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	case isErr(err, bizexport.ErrQueueFull):
		response.FailI18n(c, http.StatusTooManyRequests, response.CodeErr, err)
	case isErr(err, bizfile.ErrDriverUnavailable):
		response.FailI18n(c, http.StatusServiceUnavailable, response.CodeErr, err)
	default:
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
	}
}

// contentDisposition 生成 Content-Disposition 头：ASCII 回退名 + RFC 5987 UTF-8 编码名
func contentDisposition(disposition, filename string) string {
	fallback := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, filename)
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, fallback, url.PathEscape(filename))
}

// parseUnixParam 解析 unix 秒级时间戳查询参数（空/非法返回 false 表示不限）
func parseUnixParam(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(n, 0), true
}

func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
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
