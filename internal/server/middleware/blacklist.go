// 登录 IP 黑名单中间件：连续登录失败临时封禁该 IP，作为口令爆破的第二层防御
// （第一层为 LoginRateLimit 的 5 次/60s 频率限制；临时封禁在限流之前生效）。
// 封禁状态与计数存 Redis（见 blacklist 领域），多实例共享、重启不丢，落库后管理页可见。
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// LoginLogRecorder 登录日志记录入口（由 log 应用服务实现，异步落库）
type LoginLogRecorder interface {
	RecordLogin(ctx context.Context, l *bizlog.LoginLog)
}

// LoginGuard 登录防护入口（由 blacklist 应用服务实现，状态存 Redis，故障 fail-open）
type LoginGuard interface {
	// TempBanRemaining 返回临时封禁剩余时间；ok=false 未被封禁
	TempBanRemaining(ip string) (time.Duration, bool)
	// RecordLoginFail 登录失败计数 +1，窗口内达阈值自动临时封禁
	RecordLoginFail(ctx context.Context, ip string)
	// ResetLoginFail 登录成功清空失败计数
	ResetLoginFail(ctx context.Context, ip string)
	// HitLoginRate 登录限流计数；true 表示超出窗口上限
	HitLoginRate(ctx context.Context, ip string) bool
}

// LoginIPGuard 登录 IP 临时封禁守卫：
//   - 请求前：已被临时封禁的 IP 直接 403（提示剩余等待分钟），并记一条被拦截的登录日志；
//   - 请求后：登录成功（200）清空该 IP 失败计数；失败（401 = 密码错/账号禁用）
//     计数 +1，窗口内满阈值 → 临时封禁（阈值/窗口/时长见 biz/blacklist 常量）；
//     400（验证码错误/参数问题）不计入，避免正常用户输错验证码被误伤。
func LoginIPGuard(rec LoginLogRecorder, guard LoginGuard) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if remaining, ok := guard.TempBanRemaining(ip); ok {
			mins := int(remaining.Minutes()) + 1
			c.Header("Retry-After", strconv.Itoa(mins*60))
			response.Forbidden(c, i18n.T(c.Request.Context(), "blacklist.ip_banned", mins))
			recordBlockedLogin(c, rec, ip)
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

// recordBlockedLogin 被临时封禁拦截的尝试也入登录日志（用户名/设备端尽力从 body 提取）
func recordBlockedLogin(c *gin.Context, rec LoginLogRecorder, ip string) {
	if rec == nil {
		return
	}
	l := &bizlog.LoginLog{
		IP:        ip,
		UserAgent: truncateStr(c.GetHeader("User-Agent"), 255),
		Status:    bizlog.LoginStatusFail,
		Msg:       "IP 已被临时封禁",
		CreatedAt: time.Now(),
	}
	if body := peekBody(c); len(body) > 0 {
		var m map[string]interface{}
		if json.Unmarshal(body, &m) == nil {
			if u, ok := m["username"].(string); ok {
				l.Username = truncateStr(u, 64)
			}
			if d, ok := m["deviceType"].(string); ok {
				l.Device = truncateStr(d, 16)
			}
		}
	}
	rec.RecordLogin(context.Background(), l)
}

// ---- 管理员手工维护的持久化 IP 黑名单 ----

// Checker 持久化黑名单判定接口（由 blacklist 领域用例实现，内部为 Redis 缓存 + DB 回源）
type Checker interface {
	IsBlocked(ip string) bool
}

// IPBlacklist 持久化 IP 黑名单中间件：挂在 /api/v1 组上、JWT 之前生效，
// 命中即 403 拦截全部 /api/ 请求（不拦截静态前端资源）；
// 与上方登录失败临时封禁 LoginIPGuard 独立共存（临时封禁只拦登录接口）。
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
