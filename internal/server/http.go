// Package server 传输层：Gin HTTP Server 与路由注册。
// 切换 Kratos 时本层是唯一需要替换的层（由 proto 生成的 HTTP/gRPC server 代替）。
package server

import (
	"context"
	"errors"
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
	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	bizmerchant "github.com/smilex/smilex-admin-gin/internal/biz/merchant"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/biz/role"
	bizsession "github.com/smilex/smilex-admin-gin/internal/biz/session"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/internal/biz/user"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	appusersvc "github.com/smilex/smilex-admin-gin/internal/service/appuser"
	authsvc "github.com/smilex/smilex-admin-gin/internal/service/auth"
	blacklistsvc "github.com/smilex/smilex-admin-gin/internal/service/blacklist"
	exportsvc "github.com/smilex/smilex-admin-gin/internal/service/export"
	filesvc "github.com/smilex/smilex-admin-gin/internal/service/file"
	logsvc "github.com/smilex/smilex-admin-gin/internal/service/log"
	merchantsvc "github.com/smilex/smilex-admin-gin/internal/service/merchant"
	permsvc "github.com/smilex/smilex-admin-gin/internal/service/permission"
	rolesvc "github.com/smilex/smilex-admin-gin/internal/service/role"
	sessionsvc "github.com/smilex/smilex-admin-gin/internal/service/session"
	tenantsvc "github.com/smilex/smilex-admin-gin/internal/service/tenant"
	usersvc "github.com/smilex/smilex-admin-gin/internal/service/user"
	"github.com/smilex/smilex-admin-gin/pkg/cache"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"go.uber.org/zap"
)

// HTTPServer 聚合全部应用服务
type HTTPServer struct {
	cfg        *conf.Bootstrap
	auth       *authsvc.Service
	user       *usersvc.Service
	role       *rolesvc.Service
	perm       *permsvc.Service
	session    *sessionsvc.Service
	log        *logsvc.Service
	file       *filesvc.Service
	export     *exportsvc.Service
	blacklist  *blacklistsvc.Service
	merchant   *merchantsvc.Service
	merchantUC *bizmerchant.Usecase // 开放 API 验签中间件直连领域用例
	tenant     *tenantsvc.Service
	appuser    *appusersvc.Service
	appuserUC  *bizappuser.Usecase    // AppJWT 中间件直连领域用例（校验用户启用状态）
	appIssuer  bizappuser.TokenIssuer // AppJWT 中间件解析 app-access token
	rbacCache  *cache.TwoLevel
	rdb        *redis.Client // nonce 防重放（开放 API 验签）
	engine     *gin.Engine
	srv        *http.Server
}

// NewHTTPServer 构造并注册路由
func NewHTTPServer(cfg *conf.Bootstrap, auth *authsvc.Service, user *usersvc.Service,
	role *rolesvc.Service, perm *permsvc.Service, session *sessionsvc.Service, log *logsvc.Service,
	file *filesvc.Service, export *exportsvc.Service, blacklist *blacklistsvc.Service,
	merchant *merchantsvc.Service, merchantUC *bizmerchant.Usecase,
	tenant *tenantsvc.Service, appuser *appusersvc.Service, appuserUC *bizappuser.Usecase,
	appIssuer bizappuser.TokenIssuer, rdb *redis.Client) *HTTPServer {
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

	// RBAC 权限判定缓存：L1 30s 进程内存 + L2 60s Redis（cache.l2Enabled 可关）
	rbacCache := cache.NewTwoLevel(rdb, "rbac:", 30*time.Second, 60*time.Second, cfg.Cache.L2Enabled)

	s := &HTTPServer{cfg: cfg, auth: auth, user: user, role: role, perm: perm, session: session, log: log, file: file, export: export, blacklist: blacklist, merchant: merchant, merchantUC: merchantUC, tenant: tenant, appuser: appuser, appuserUC: appuserUC, appIssuer: appIssuer, rbacCache: rbacCache, rdb: rdb, engine: e}
	s.registerRoutes()
	s.registerStatic()

	s.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: e,
	}
	return s
}

func (s *HTTPServer) registerRoutes() {
	// 持久化 IP 黑名单在 JWT 之前拦截全部 /api/ 请求（静态前端资源不经过此组）
	v1 := s.engine.Group("/api/v1", middleware.IPBlacklist(s.blacklist.Checker()))

	// ---- 公开接口 ----
	authg := v1.Group("/auth")
	{
		// 图形验证码：无需鉴权，登录页拉取
		authg.GET("/captcha", func(c *gin.Context) {
			vo, err := s.auth.GenerateCaptcha()
			if err != nil {
				response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
				return
			}
			response.OK(c, vo)
		})
		// 登录接口：IP 临时封禁（连续失败拉黑）→ 频率限制 → 登录，防口令爆破
		authg.POST("/login", middleware.LoginIPGuard(s.log, s.blacklist.LoginGuard()), middleware.LoginRateLimit(s.blacklist.LoginGuard()), func(c *gin.Context) {
			var req authsvc.LoginRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
				return
			}
			// 登录环境注入（建立会话用；UA 截断防止超长存储）
			req.IP = c.ClientIP()
			req.UserAgent = truncate(c.GetHeader("User-Agent"), 255)
			tp, err := s.auth.Login(c.Request.Context(), req)
			// 登录尝试（成功/失败）均落登录日志（异步，不影响登录响应）
			loginStatus := bizlog.LoginStatusSuccess
			loginMsg := ""
			if err != nil {
				loginStatus = bizlog.LoginStatusFail
				loginMsg = err.Error()
			}
			s.log.RecordLogin(c.Request.Context(), &bizlog.LoginLog{
				Username: req.Username, IP: req.IP, UserAgent: req.UserAgent,
				Device: bizsession.NormalizeDevice(req.DeviceType),
				Status: loginStatus, Msg: loginMsg,
			})
			if err != nil {
				// 验证码错误属入参问题，返回 400 便于前端区分提示并自动刷新
				if errors.Is(err, bizauth.ErrCaptcha) {
					response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
					return
				}
				response.FailI18n(c, http.StatusUnauthorized, response.CodeUnauthorized, err)
				return
			}
			response.OK(c, tp)
		})
		authg.POST("/refresh", func(c *gin.Context) {
			var req authsvc.RefreshRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
				return
			}
			tp, err := s.auth.Refresh(c.Request.Context(), req)
			if err != nil {
				response.FailI18n(c, http.StatusUnauthorized, response.CodeUnauthorized, err)
				return
			}
			response.OK(c, tp)
		})
	}

	// ---- 应用用户认证（与后台账号体系 typ 隔离；无验证码、无服务端会话） ----
	// 登录接口挂与后台登录同款的 IP 临时封禁 + 频率限制防护，防口令爆破
	appauthg := v1.Group("/app-auth")
	{
		appauthg.POST("/login", middleware.LoginIPGuard(s.log, s.blacklist.LoginGuard()), middleware.LoginRateLimit(s.blacklist.LoginGuard()), s.appLogin)
		appauthg.POST("/refresh", s.appRefresh)
	}

	// 应用用户自身数据接口：仅 AppJWT 认证（查库校验启用状态），不做 RBAC
	appAuth := v1.Group("/app-auth", middleware.AppJWT(s.appIssuer, s.appuserUC))
	{
		appAuth.GET("/profile", s.appProfile)
		appAuth.PUT("/password", s.appChangePassword)
	}

	// ---- 自身数据接口：仅 JWT 认证，不做 RBAC ----
	// profile 是本人信息、menus 是已按角色过滤的本人菜单树、logout 无服务端状态，均无越权面；
	// 若纳入默认拒绝的 RBAC，仅绑定了菜单/按钮权限的普通用户登录后即 403 白屏。
	// OpLog 自动审计写请求（登出/改资料/改密码）。
	basic := v1.Group("", middleware.JWT(s.auth), middleware.OpLog(s.log))
	{
		// 登出：吊销当前会话，token 立即失效
		basic.POST("/auth/logout", func(c *gin.Context) {
			if sub := middleware.Subject(c); sub != nil {
				if err := s.auth.Logout(c.Request.Context(), sub.SessionID); err != nil {
					logger.Warn("logout revoke session failed", zap.Error(err))
				}
			}
			response.OK(c, nil)
		})
		basic.GET("/auth/profile", func(c *gin.Context) {
			sub := middleware.Subject(c)
			vo, err := s.auth.Profile(c.Request.Context(), sub.UserID)
			if err != nil {
				response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
				return
			}
			response.OK(c, vo)
		})
		// 本人更新昵称/邮箱
		basic.PUT("/auth/profile", func(c *gin.Context) {
			sub := middleware.Subject(c)
			var req authsvc.UpdateProfileRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
				return
			}
			vo, err := s.auth.UpdateProfile(c.Request.Context(), sub.UserID, req)
			if err != nil {
				response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
				return
			}
			response.OK(c, vo)
		})
		// 本人修改密码（校验旧密码；成功后吊销其他端会话，当前端保持登录）
		basic.PUT("/auth/password", func(c *gin.Context) {
			sub := middleware.Subject(c)
			var req authsvc.ChangePasswordRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
				return
			}
			sid := ""
			if sub != nil {
				sid = sub.SessionID
			}
			if err := s.auth.ChangePassword(c.Request.Context(), sub.UserID, req, sid); err != nil {
				if errors.Is(err, bizauth.ErrInvalidCredentials) {
					response.BadRequest(c, i18n.T(c.Request.Context(), "profile.wrong_old_password"))
					return
				}
				response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
				return
			}
			response.OK(c, nil)
		})
		// 当前用户可见菜单树
		basic.GET("/menus", func(c *gin.Context) {
			sub := middleware.Subject(c)
			tree, err := s.perm.UserMenuTree(c.Request.Context(), sub.UserID)
			if err != nil {
				response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
				return
			}
			response.OK(c, tree)
		})
		// 菜单搜索（顶栏命令面板）：当前用户可见菜单内按关键词模糊匹配
		basic.GET("/menus/search", func(c *gin.Context) {
			sub := middleware.Subject(c)
			kw := strings.TrimSpace(c.Query("kw"))
			hits, err := s.perm.SearchUserMenus(c.Request.Context(), sub.UserID, kw)
			if err != nil {
				response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
				return
			}
			if hits == nil {
				hits = []*bizperm.MenuHit{}
			}
			response.OK(c, hits)
		})

		// 异步导出：记录归属当前用户，列表/下载/删除均强制按 sub.UserID 过滤（biz 层校验），
		// 与 profile/menus 同属自身数据接口，故仅 JWT 不走 RBAC；导出入口（POST */export）在 protected 组按按钮权限点控制
		exports := basic.Group("/exports")
		{
			// recent=1 返回近期 5 条（任务浮层轮询）；否则分页返回本人记录
			exports.GET("", s.listExports)
			// 下载：云存储 302 到预签名 URL，本地存储后端代理流式输出
			exports.GET("/:id/download", s.downloadExport)
			exports.DELETE("/:id", s.deleteExport)
		}
	}

	// ---- 受保护接口：JWT -> 操作日志（RBAC 拒绝的尝试也记录）-> RBAC ----
	protected := v1.Group("", middleware.JWT(s.auth), middleware.OpLog(s.log), middleware.RBAC(s.auth, s.rbacCache))

	users := protected.Group("/users")
	{
		users.GET("", s.listUsers)
		users.POST("", s.createUser)
		users.GET("/:id", s.getUser)
		users.PUT("/:id", s.updateUser)
		users.DELETE("/:id", s.deleteUser)
		users.PUT("/:id/roles", s.setUserRoles)
		users.PUT("/:id/password", s.resetUserPassword)
		// 导出用户列表（查询条件透传 query，与列表页一致）
		users.POST("/export", func(c *gin.Context) { s.submitExport(c, "user") })
		// 踢某用户全部端下线（挂在 users 资源下，避免与 /online-users/:sid 路由冲突）
		users.DELETE("/:id/sessions", s.kickUserSessions)
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

	onlines := protected.Group("/online-users")
	{
		onlines.GET("", s.listOnlineUsers)
		onlines.DELETE("/:sid", s.kickOnlineSession)
	}

	loginLogs := protected.Group("/login-logs")
	{
		loginLogs.GET("", s.listLoginLogs)
		loginLogs.DELETE("", s.clearLoginLogs)
		loginLogs.POST("/export", func(c *gin.Context) { s.submitExport(c, "login_log") })
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
		// 下载/预览：云存储 302 到预签名 URL，本地存储后端代理流式输出
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

	merchants := protected.Group("/merchants")
	{
		merchants.GET("", s.listMerchants)
		merchants.POST("", s.createMerchant)
		merchants.GET("/:id", s.getMerchant)
		merchants.PUT("/:id", s.updateMerchant)
		merchants.DELETE("/:id", s.deleteMerchant)
		// 重置密钥（旧 secret 立即失效，新明文仅此一次返回）
		merchants.PUT("/:id/secret", s.resetMerchantSecret)
		merchants.PUT("/:id/status", s.setMerchantStatus)
	}

	mapiLogs := protected.Group("/merchant-api-logs")
	{
		mapiLogs.GET("", s.listMerchantAPILogs)
	}

	tenants := protected.Group("/tenants")
	{
		tenants.GET("", s.listTenants)
		tenants.POST("", s.createTenant)
		tenants.GET("/:id", s.getTenant)
		tenants.PUT("/:id", s.updateTenant)
		tenants.DELETE("/:id", s.deleteTenant)
		tenants.PUT("/:id/status", s.setTenantStatus)
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

	// ---- 开放 API：IP 黑名单 → 商户 HMAC 验签（时间戳偏差 + nonce 防重放） ----
	open := s.engine.Group("/open-api/v1",
		middleware.IPBlacklist(s.blacklist.Checker()),
		middleware.OpenAPIAuth(s.merchantUC, s.rdb, s.cfg.OpenAPI),
	)
	{
		// 探活/凭证自检：返回当前商户脱敏信息
		open.GET("/ping", func(c *gin.Context) {
			response.OK(c, merchantsvc.ToVO(middleware.MerchantFromContext(c)))
		})
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

// ---- 用户 ----

type listResult struct {
	List interface{} `json:"list"`
	Page interface{} `json:"page"`
}

func (s *HTTPServer) listUsers(c *gin.Context) {
	page, size := pageParams(c)
	q := user.Query{Username: c.Query("username")}
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	users, pg, err := s.user.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: users, Page: pg})
}

func (s *HTTPServer) createUser(c *gin.Context) {
	var req usersvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.user.Create(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) getUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.user.Get(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) updateUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req usersvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.user.Update(user.WithOperator(c.Request.Context(), middleware.Subject(c).UserID), id, req); err != nil {
		s.userErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.user.Delete(c.Request.Context(), id); err != nil {
		s.userErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setUserRoles(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req usersvc.SetRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.user.SetRoles(user.WithOperator(c.Request.Context(), middleware.Subject(c).UserID), id, req); err != nil {
		s.userErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) resetUserPassword(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req usersvc.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.user.ResetPassword(user.WithOperator(c.Request.Context(), middleware.Subject(c).UserID), id, req); err != nil {
		s.userErr(c, err)
		return
	}
	response.OK(c, nil)
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

// ---- helpers ----

// ---- 在线用户 ----

func (s *HTTPServer) listOnlineUsers(c *gin.Context) {
	page, size := pageParams(c)
	q := bizsession.Query{Username: c.Query("username"), Device: c.Query("device")}
	currentSid := ""
	if sub := middleware.Subject(c); sub != nil {
		currentSid = sub.SessionID
	}
	vos, pg, err := s.session.List(c.Request.Context(), q, page, size, currentSid)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: vos, Page: pg})
}

func (s *HTTPServer) kickOnlineSession(c *gin.Context) {
	sid := c.Param("sid")
	if sid == "" {
		response.BadRequest(c, "invalid sid")
		return
	}
	operatorID := uint(0)
	if sub := middleware.Subject(c); sub != nil {
		operatorID = sub.UserID
	}
	if err := s.session.Kick(c.Request.Context(), sid, operatorID); err != nil {
		s.sessionErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) kickUserSessions(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	operatorID := uint(0)
	if sub := middleware.Subject(c); sub != nil {
		operatorID = sub.UserID
	}
	if _, err := s.session.KickUser(c.Request.Context(), id, operatorID); err != nil {
		s.sessionErr(c, err)
		return
	}
	response.OK(c, nil)
}

// sessionErr 会话操作错误映射：超管保护返回 403，会话不存在返回 404，其余返回 400
func (s *HTTPServer) sessionErr(c *gin.Context, err error) {
	if errors.Is(err, user.ErrSuperAdminProtected) {
		response.FailI18n(c, http.StatusForbidden, response.CodeForbidden, err)
		return
	}
	if errors.Is(err, bizsession.ErrSessionNotFound) {
		response.NotFound(c, i18n.T(c.Request.Context(), "session.not_found"))
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

// userErr 用户操作错误映射：超管保护类返回 403，其余返回 400
func (s *HTTPServer) userErr(c *gin.Context, err error) {
	if errors.Is(err, user.ErrSuperAdminProtected) || errors.Is(err, user.ErrDeleteSuperAdmin) {
		response.FailI18n(c, http.StatusForbidden, response.CodeForbidden, err)
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

// roleErr 角色操作错误映射：超管角色保护类返回 403，其余返回 400
func (s *HTTPServer) roleErr(c *gin.Context, err error) {
	if errors.Is(err, role.ErrSuperRoleLocked) {
		response.FailI18n(c, http.StatusForbidden, response.CodeForbidden, err)
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

// ---- 日志 ----

// logListResult 日志列表响应（附保留天数，前端展示保留说明用）
type logListResult struct {
	List          interface{} `json:"list"`
	Page          interface{} `json:"page"`
	RetentionDays int         `json:"retention_days"`
}

func (s *HTTPServer) listLoginLogs(c *gin.Context) {
	page, size := pageParams(c)
	q := bizlog.LoginLogQuery{Username: c.Query("username"), IP: c.Query("ip")}
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	if t, ok := parseUnixParam(c.Query("start")); ok {
		q.Start = t
	}
	if t, ok := parseUnixParam(c.Query("end")); ok {
		q.End = t
	}
	logs, pg, err := s.log.ListLoginLogs(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, logListResult{List: logs, Page: pg, RetentionDays: s.log.RetentionDays()})
}

func (s *HTTPServer) clearLoginLogs(c *gin.Context) {
	n, err := s.log.ClearLoginLogs(c.Request.Context())
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, gin.H{"deleted": n})
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
	// 云存储：鉴权通过后 302 到短时效预签名 URL
	if d.URL != "" {
		c.Redirect(http.StatusFound, d.URL)
		return
	}
	// 本地存储：后端代理流式输出
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
	case errors.Is(err, bizfile.ErrFileNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case errors.Is(err, bizfile.ErrFileTooLarge):
		// 上限参数按当前配置渲染（biz 层只返回哨兵错误）
		response.BadRequest(c, i18n.T(c.Request.Context(), "file.too_large", s.cfg.Storage.MaxSizeMB))
	case errors.Is(err, bizfile.ErrFileTypeDenied):
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	case errors.Is(err, bizfile.ErrDriverUnavailable):
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

// ---- 商户（开放 API 授权） ----

func (s *HTTPServer) listMerchants(c *gin.Context) {
	page, size := pageParams(c)
	q := bizmerchant.Query{
		Name:   strings.TrimSpace(c.Query("name")),
		Code:   strings.TrimSpace(c.Query("code")),
		AppKey: strings.TrimSpace(c.Query("app_key")),
	}
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	list, pg, err := s.merchant.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createMerchant(c *gin.Context) {
	var req merchantsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.merchant.Create(c.Request.Context(), req)
	if err != nil {
		s.merchantErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) getMerchant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.merchant.Get(c.Request.Context(), id)
	if err != nil {
		s.merchantErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) updateMerchant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req merchantsvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.merchant.Update(c.Request.Context(), id, req); err != nil {
		s.merchantErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteMerchant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.merchant.Delete(c.Request.Context(), id); err != nil {
		s.merchantErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) resetMerchantSecret(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.merchant.ResetSecret(c.Request.Context(), id)
	if err != nil {
		s.merchantErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) setMerchantStatus(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req merchantsvc.SetStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.merchant.SetStatus(c.Request.Context(), id, req); err != nil {
		s.merchantErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) listMerchantAPILogs(c *gin.Context) {
	page, size := pageParams(c)
	q := bizmerchant.APILogQuery{AppKey: c.Query("app_key"), Path: c.Query("path")}
	if v := c.Query("status_code"); v != "" {
		if sc, err := strconv.Atoi(v); err == nil {
			q.StatusCode = &sc
		}
	}
	if t, ok := parseUnixParam(c.Query("start")); ok {
		q.Start = t
	}
	if t, ok := parseUnixParam(c.Query("end")); ok {
		q.End = t
	}
	logs, pg, err := s.merchant.ListAPILogs(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: logs, Page: pg})
}

// merchantErr 商户操作错误映射：不存在 404，其余 400
func (s *HTTPServer) merchantErr(c *gin.Context, err error) {
	if errors.Is(err, bizmerchant.ErrMerchantNotFound) {
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
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

// tenantErr 租户操作错误映射：不存在 404，存在关联应用用户 409，其余 400
func (s *HTTPServer) tenantErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, biztenant.ErrTenantNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case errors.Is(err, biztenant.ErrTenantInUse):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	default:
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
	if errors.Is(err, bizappuser.ErrAppUserNotFound) {
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
		if errors.Is(err, bizappuser.ErrBadCredentials) {
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
		case errors.Is(err, bizexport.ErrQueueFull):
			response.FailI18n(c, http.StatusTooManyRequests, response.CodeErr, err)
		case errors.Is(err, bizexport.ErrUnsupportedBiz):
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
	// 云存储：鉴权通过后 302 到短时效预签名 URL
	if d.URL != "" {
		c.Redirect(http.StatusFound, d.URL)
		return
	}
	// 本地存储：后端代理流式输出（强制 attachment + nosniff，CSV 不内联渲染）
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
	case errors.Is(err, bizexport.ErrNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case errors.Is(err, bizexport.ErrNotOwner):
		response.FailI18n(c, http.StatusForbidden, response.CodeForbidden, err)
	case errors.Is(err, bizexport.ErrNotReady):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	case errors.Is(err, bizexport.ErrQueueFull):
		response.FailI18n(c, http.StatusTooManyRequests, response.CodeErr, err)
	case errors.Is(err, bizfile.ErrDriverUnavailable):
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

// truncate 按字节长度截断（UA 摘要存储用）
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
