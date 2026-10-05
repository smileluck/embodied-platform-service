// Package middleware HTTP 中间件：平台认证（token 自省 + 本地准入）、RBAC 鉴权、CORS。
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	"github.com/smilex/smilex-admin-gin/internal/biz/auth"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
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

// CORS 跨域资源 sharing：
//   - 未配置白名单（默认）：同源模式，不下发任何 CORS 头。SPA 由后端同源托管、
//     本地开发走 vite 代理（亦同源），均不依赖跨域头；
//   - 配置来源列表：仅 Origin 命中时回显该来源并允许携带凭证（Authorization）；
//   - 显式配置 ["*"]：恢复通配（历史行为），通配与凭证互斥，不下发 Allow-Credentials。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	wildcard := false
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(o), "/"))
		if o == "" {
			continue
		}
		if o == "*" {
			wildcard = true
			continue
		}
		allowed[strings.ToLower(o)] = struct{}{}
	}
	if !wildcard && len(allowed) == 0 {
		return func(c *gin.Context) { c.Next() }
	}
	const methods = "GET, POST, PUT, DELETE, OPTIONS"
	const headers = "Origin, Content-Type, Authorization"
	return func(c *gin.Context) {
		h := c.Writer.Header()
		if wildcard {
			h.Set("Access-Control-Allow-Origin", "*")
			h.Set("Access-Control-Allow-Methods", methods)
			h.Set("Access-Control-Allow-Headers", headers)
		} else {
			// 命中判定与缓存正确性都依赖 Origin，凡走白名单分支必带 Vary
			origin := strings.ToLower(c.GetHeader("Origin"))
			h.Add("Vary", "Origin")
			if origin == "" {
				c.Next()
				return
			}
			if _, ok := allowed[origin]; !ok {
				// 未命中白名单：不下发跨域头，浏览器侧自然拒绝跨域读取；同源请求不受影响
				c.Next()
				return
			}
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", methods)
			h.Set("Access-Control-Allow-Headers", headers)
		}
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
		if st.Known && !st.Admitted {
			// 准入已在平台侧暂停（本端开关写回的事实源；TTL 内同步拒绝）
			response.Forbidden(c, "account not admitted to this console")
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

const (
	ctxAppAuthSubjectKey = "appauth.subject"
	ctxAppAuthTenantKey  = "appauth.tenant"
	ctxAppAuthTokenKey   = "appauth.token"
)

// TenantResolver AppAuth 租户闸门所需的最小只读接口（*biztenant.Usecase 满足）
type TenantResolver interface {
	GetByPlatformID(ctx context.Context, platformID uint) (*biztenant.Tenant, error)
}

// AppAuth 平台应用用户（C 端）认证：Bearer app-access token →（缓存）平台
// /app-auth/profile 自省 → 租户闸门。与 PlatformAuth 同骨架但闸门不同：
//   - token 由 App 直连平台 /app-auth 登录取得（双用于平台与本系统），本系统对 C 端
//     不碰账密、不签发、不换签、无本地投影/RBAC；
//   - 自省结果按 token 哈希缓存 30-60s：平台侧禁用按请求查库即时生效，本系统在缓存
//     TTL 内感知（与 B 端同一权衡）；
//   - 闸门（归属即准入）：请求头 X-Tenant-ID（平台租户 ID，全链路唯一口径）必须
//     ∈ 用户归属集（平台 app_user_tenants）且本地租户（tenants.platform_id）已同步，
//     否则 403；缺失/非法 400；两种 403 同文案不泄露是「未归属」还是「未同步」；
//   - 平台不可达 fail-closed（503），不降级放行。
func AppAuth(ids auth.AppIdentitySource, tenants TenantResolver, idCache *cache.TwoLevel) gin.HandlerFunc {
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
			sub, err := ids.AppProfile(ctx, token)
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
				response.ServiceUnavailable(c, "platform unavailable")
			default:
				response.ServerError(c, "authenticate failed")
			}
			c.Abort()
			return
		}
		var sub auth.AppSubject
		if err := json.Unmarshal([]byte(val), &sub); err != nil || sub.UserID == 0 {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		// 租户闸门：X-Tenant-ID = 平台租户 ID
		raw := strings.TrimSpace(c.GetHeader("X-Tenant-ID"))
		tid, perr := strconv.ParseUint(raw, 10, 64)
		if raw == "" || perr != nil || tid == 0 {
			response.BadRequest(c, "missing or invalid X-Tenant-ID header")
			c.Abort()
			return
		}
		platformTenantID := uint(tid)
		if !containsUint(sub.TenantIDs, platformTenantID) {
			response.Forbidden(c, "tenant not accessible for this app user")
			c.Abort()
			return
		}
		tn, err := tenants.GetByPlatformID(c.Request.Context(), platformTenantID)
		if err != nil || tn == nil || tn.ID == 0 || !tn.Enabled() {
			// 本地未同步/已删/已停用：与未归属同语义 403（不泄露细节）；
			// 停用租户挡住其 App 流量（闸门读本地投影，停用经对账回流跟随平台）
			response.Forbidden(c, "tenant not accessible for this app user")
			c.Abort()
			return
		}

		c.Set(ctxAppAuthSubjectKey, &sub)
		c.Set(ctxAppAuthTenantKey, tn)
		c.Set(ctxAppAuthTokenKey, token)
		c.Next()
	}
}

// AppAuthSubject 从 context 取平台应用用户主体（AppAuth 中间件之后可用；
// UserID 为平台 app_user ID，TenantIDs 为平台租户 ID 集）
func AppAuthSubject(c *gin.Context) *auth.AppSubject {
	if v, ok := c.Get(ctxAppAuthSubjectKey); ok {
		if s, ok := v.(*auth.AppSubject); ok {
			return s
		}
	}
	return nil
}

// AppAuthTenant 从 context 取本次请求的本地租户上下文（AppAuth 中间件之后可用；
// 由 X-Tenant-ID（平台租户 ID）解析命中）
func AppAuthTenant(c *gin.Context) *biztenant.Tenant {
	if v, ok := c.Get(ctxAppAuthTenantKey); ok {
		if t, ok := v.(*biztenant.Tenant); ok {
			return t
		}
	}
	return nil
}

// AppAuthToken 从 context 取当前请求的 app-access token（代理平台 C 端接口用）
func AppAuthToken(c *gin.Context) string {
	if v, ok := c.Get(ctxAppAuthTokenKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func containsUint(ids []uint, id uint) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
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
