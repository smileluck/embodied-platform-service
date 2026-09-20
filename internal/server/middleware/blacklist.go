// 登录 IP 黑名单中间件：连续登录失败临时封禁该 IP，作为口令爆破的第二层防御
// （第一层为 security.go 的 LoginRateLimit 频率限制；临时封禁在限流之前生效）。
// 管理端登录已委外给平台（其自带防护），本中间件仅守护应用用户登录（app-auth）。
// 封禁状态与计数存 Redis（见 blacklist 领域），多实例共享、重启不丢，落库后管理页可见。
package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// LoginGuard 登录防护入口（由 blacklist 应用服务实现，状态存 Redis，故障 fail-open）
type LoginGuard interface {
	// TempBanRemaining 返回临时封禁剩余时间；ok=false 未被封禁
	TempBanRemaining(ip string) (time.Duration, bool)
	// RecordLoginFail 登录失败计数 +1，窗口内达阈值自动临时封禁
	RecordLoginFail(ctx context.Context, ip string)
	// ResetLoginFail 登录成功清空失败计数
	ResetLoginFail(ctx context.Context, ip string)
}

// LoginIPGuard 登录 IP 临时封禁守卫：
//   - 请求前：已被临时封禁的 IP 直接 403（提示剩余等待分钟）；
//   - 请求后：登录成功（200）清空该 IP 失败计数；失败（401 = 密码错/账号禁用）
//     计数 +1，窗口内满阈值 → 临时封禁（阈值/窗口/时长见 biz/blacklist 常量）；
//     400（参数问题）不计入。
func LoginIPGuard(guard LoginGuard) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if remaining, ok := guard.TempBanRemaining(ip); ok {
			mins := int(remaining.Minutes()) + 1
			c.Header("Retry-After", strconv.Itoa(mins*60))
			response.Forbidden(c, i18n.T(c.Request.Context(), "blacklist.ip_banned", mins))
			c.Abort()
			return
		}

		c.Next()

		switch c.Writer.Status() {
		case http.StatusOK:
			guard.ResetLoginFail(c.Request.Context(), ip)
		case http.StatusUnauthorized:
			guard.RecordLoginFail(c.Request.Context(), ip)
		}
	}

}

// ---- 管理员手工维护的持久化 IP 黑名单 ----

// Checker 持久化黑名单判定接口（由 blacklist 领域用例实现，内部为 Redis 缓存 + DB 回源）
type Checker interface {
	IsBlocked(ip string) bool
}

// IPBlacklist 持久化 IP 黑名单中间件：挂在 /api/v1 组上、认证之前生效，
// 命中即 403 拦截全部 /api/ 请求（不拦截静态前端资源）。
func IPBlacklist(chk Checker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if chk != nil && chk.IsBlocked(c.ClientIP()) {
			response.Forbidden(c, i18n.T(c.Request.Context(), "blacklist.ip_blocked"))
			c.Abort()
			return
		}
		c.Next()
	}
}
