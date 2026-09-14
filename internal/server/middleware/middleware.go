// Package middleware HTTP 中间件：JWT 认证、RBAC 鉴权、CORS。
package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/biz/auth"
	authsvc "github.com/smilex/smilex-admin-gin/internal/service/auth"
	"github.com/smilex/smilex-admin-gin/pkg/cache"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// I18n 按 Accept-Language 头识别请求语言并注入 context（须在全局链最前注册，
// 保证后续中间件与 handler 都能从 context 取到语言）
func I18n() gin.HandlerFunc {
	return func(c *gin.Context) {
		l := i18n.Detect(c.GetHeader("Accept-Language"))
		c.Set("locale", string(l))
		c.Request = c.Request.WithContext(i18n.WithLocale(c.Request.Context(), l))
		c.Next()
	}
}

// CORS 跨域
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

const ctxSubjectKey = "auth.subject"

// JWT 认证：Bearer token -> Subject 存入 context。
// 会话校验：token 携带 sid，仅当对应会话存活（未被吊销/顶替/过期）时放行；
// Redis 故障时 fail-closed，会话一律视为无效。
func JWT(authSvc *authsvc.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || token == "" {
			response.Unauthorized(c, "missing bearer token")
			c.Abort()
			return
		}
		s, err := authSvc.ParseSubject(token)
		if err != nil {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}
		if s.SessionID == "" || !authSvc.ValidateSession(c.Request.Context(), s.SessionID) {
			response.Unauthorized(c, "session revoked")
			c.Abort()
			return
		}
		c.Set(ctxSubjectKey, s)
		c.Next()
	}
}

// Subject 从 context 取认证主体（JWT 中间件之后可用）
func Subject(c *gin.Context) *auth.Subject {
	if v, ok := c.Get(ctxSubjectKey); ok {
		if s, ok := v.(*auth.Subject); ok {
			return s
		}
	}
	return nil
}

const ctxAppSubjectKey = "app.subject"

// appSubject 应用用户认证主体（app-access token 载荷，无会话 ID）
type appSubject struct {
	UserID   uint
	Username string
}

// AppJWT 应用用户认证：Bearer token -> 校验 app-access typ -> 用户存在且启用 -> 注入 context。
// 不做 RBAC、不查 Redis 会话（应用用户无服务端会话状态，禁用即时生效依赖每次查库）。
func AppJWT(issuer bizappuser.TokenIssuer, uc *bizappuser.Usecase) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || token == "" {
			response.Unauthorized(c, "missing bearer token")
			c.Abort()
			return
		}
		uid, username, err := issuer.ParseAppAccessToken(token)
		if err != nil {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}
		u, err := uc.Profile(c.Request.Context(), uid)
		if err != nil || !u.Enabled() {
			response.Unauthorized(c, "account disabled or not found")
			c.Abort()
			return
		}
		c.Set(ctxAppSubjectKey, &appSubject{UserID: uid, Username: username})
		c.Next()
	}
}

// AppSubject 从 context 取应用用户认证主体（AppJWT 中间件之后可用）
func AppSubject(c *gin.Context) *appSubject {
	if v, ok := c.Get(ctxAppSubjectKey); ok {
		if s, ok := v.(*appSubject); ok {
			return s
		}
	}
	return nil
}

// RBAC 接口鉴权（二级缓存：L1 进程内存 + L2 Redis，减少查库；一致性由短 TTL 兜底）
func RBAC(authSvc *authsvc.Service, cache *cache.TwoLevel) gin.HandlerFunc {
	return func(c *gin.Context) {
		s := Subject(c)
		if s == nil {
			response.Unauthorized(c, "unauthenticated")
			c.Abort()
			return
		}
		key := fmt.Sprintf("%d|%s|%s", s.UserID, c.Request.Method, c.Request.URL.Path)
		// Load 内部 singleflight 合并并发回源，miss 时不会打爆数据库
		val, err := cache.Load(c.Request.Context(), key, func(ctx context.Context) (string, error) {
			if authSvc.Authorize(ctx, s.UserID, c.Request.Method, c.Request.URL.Path) {
				return "1", nil
			}
			return "0", nil
		})
		if err != nil {
			response.ServerError(c, "authorize failed")
			c.Abort()
			return
		}
		if val != "1" {
			response.Forbidden(c, "permission denied")
			c.Abort()
		}
	}
}
