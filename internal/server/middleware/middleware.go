// Package middleware HTTP 中间件：平台认证（token 自省 + 本地准入）、RBAC 鉴权、CORS。
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
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

const ctxTokenKey = "auth.token"

// PlatformAuth 平台认证：Bearer 平台 token →（缓存）平台 profile 自省 → 本地准入投影校验。
//
// 平台是唯一身份源（token 由前端直调平台登录取得，双用于两系统）：
//   - 自省结果（平台用户 ID/用户名/已准入商户等身份）按 token 哈希缓存 30-60s，
//     平台侧吊销/改密/解绑在缓存 TTL 内感知（与平台统一账号决策一致）；
//   - 准入状态每请求查本地投影：禁用/删除投影后立即 403（无缓存，即时生效）；
//   - 商户成员闸门：平台侧已解绑本商户（merchant_users 无绑定）一律 403——
//     平台成员关系是准入的唯一事实源，本地投影只是缓存与开关；
//   - 平台标记的商户管理员首登自动建投影并绑内置「商户管理员」角色；
//     标记与本地绑定每请求对账自愈（撤标记 30-60s 内回收）；
//   - 平台不可达时 fail-closed（503），不降级放行；本商户身份未识别时
//     跳过成员闸门与管理员自愈（不因未知误拒/误绑）。
func PlatformAuth(authSrv *authsvc.Service, admissionUC *bizadmission.Usecase, idCache *cache.TwoLevel) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || token == "" {
			response.Unauthorized(c, "missing bearer token")
			c.Abort()
			return
		}
		sum := sha256.Sum256([]byte(token))
		key := "t:" + hex.EncodeToString(sum[:16])
		val, err := idCache.Load(c.Request.Context(), key, func(ctx context.Context) (string, error) {
			sub, err := authSrv.Introspect(ctx, token)
			if err != nil {
				return "", err
			}
			b, err := json.Marshal(sub)
			if err != nil {
				return "", err
			}
			return string(b), nil
		})
		if err != nil {
			// loader 出错不回填缓存；按哨兵错误分类响应
			switch {
			case errors.Is(err, auth.ErrInvalidToken):
				response.Unauthorized(c, "invalid or expired token")
			case errors.Is(err, auth.ErrPlatformUnavailable):
				response.ServerError(c, "platform unavailable")
			default:
				response.ServerError(c, "authenticate failed")
			}
			c.Abort()
			return
		}
		var sub auth.Subject
		if err := json.Unmarshal([]byte(val), &sub); err != nil || sub.UserID == 0 {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		// 平台身份与本商户的关系（成员/管理员；身份未知时跳过闸门与自愈）
		st := admissionUC.ResolveMerchantStatus(c.Request.Context(), sub.Merchants)
		if st.Known && !st.Member {
			response.Forbidden(c, "account not bound to this merchant")
			c.Abort()
			return
		}

		// 本地准入：未建投影一律 403（唯一例外：平台标记的商户管理员首登自动准入）
		proj, err := admissionUC.Admission(c.Request.Context(), sub.UserID)
		if err != nil {
			response.ServerError(c, "admission lookup failed")
			c.Abort()
			return
		}
		if proj == nil && st.Known && st.IsAdmin {
			proj, err = admissionUC.EnsureMerchantAdmin(c.Request.Context(), sub.UserID, sub.Username, sub.Nickname)
			if err != nil {
				response.ServerError(c, "merchant admin admission failed")
				c.Abort()
				return
			}
		}
		if proj == nil || !proj.Enabled {
			response.Forbidden(c, "account not admitted to this console")
			c.Abort()
			return
		}
		if st.Known {
			// 管理员标记对账（绑/解内置角色）；失败不阻断请求（RBAC 按本地实际绑定判定，偏保守）
			_ = admissionUC.ReconcileMerchantAdmin(c.Request.Context(), proj, st.IsAdmin)
		}

		c.Set(ctxSubjectKey, &sub)
		c.Set(ctxTokenKey, token)
		c.Next()
	}
}

// Subject 从 context 取认证主体（PlatformAuth 中间件之后可用；UserID 为平台用户 ID）
func Subject(c *gin.Context) *auth.Subject {
	if v, ok := c.Get(ctxSubjectKey); ok {
		if s, ok := v.(*auth.Subject); ok {
			return s
		}
	}
	return nil
}

// Token 从 context 取当前请求的平台 token（代理平台自身数据接口用）
func Token(c *gin.Context) string {
	if v, ok := c.Get(ctxTokenKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
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

// RBAC 接口鉴权（二级缓存：L1 进程内存 + L2 Redis，减少查库；一致性由短 TTL 兜底 +
// 准入/角色/权限变更时整体失效）。UserID 为平台用户 ID。
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
