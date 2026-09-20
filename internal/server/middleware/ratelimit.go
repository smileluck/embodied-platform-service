// 通用固定窗口限流中间件（Redis INCR 计数，多实例共享）。
// 与登录限流同策略：Redis 故障 fail-open 放行并告警 —— 限流是防御层，不应因存储故障阻断业务；
// 认证类接口的可用性底线由 JWT 会话校验（fail-closed）兜底。
package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"go.uber.org/zap"
)

// RateLimitConfig 固定窗口限流配置
type RateLimitConfig struct {
	KeyPrefix  string        // Redis key 前缀，须以 ":" 结尾（如 "rl:agent-chat:"）
	Max        int64         // 窗口内最大放行次数，超出返回 429
	Window     time.Duration // 固定窗口时长
	ByUser     bool          // true 按认证主体（UserID）计数，须挂载在 JWT 之后；false 按客户端 IP
	MessageKey string        // 超限提示的 i18n 词条 key
}

// NewRateLimit 通用限流中间件：按 IP 或认证用户做固定窗口计数。
// 注意 key 前缀与既有语义的衔接：登录接口沿用 "bl:rl:"，黑名单提前解封时联动清零（见 blacklist 领域 Delete）。
func NewRateLimit(rdb *redis.Client, cfg RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var subject string
		if cfg.ByUser {
			s := Subject(c)
			if s == nil {
				response.Unauthorized(c, "unauthenticated")
				c.Abort()
				return
			}
			subject = strconv.FormatUint(uint64(s.UserID), 10)
		} else {
			subject = c.ClientIP()
		}
		key := cfg.KeyPrefix + subject
		n, err := rdb.Incr(c.Request.Context(), key).Result()
		if err != nil {
			logger.Warn("rate limit incr failed, fail-open", zap.String("key", key), zap.Error(err))
			c.Next()
			return
		}
		if n == 1 {
			if err := rdb.Expire(c.Request.Context(), key, cfg.Window).Err(); err != nil {
				logger.Warn("rate limit expire failed", zap.String("key", key), zap.Error(err))
			}
		}
		if n > cfg.Max {
			c.Header("Retry-After", strconv.Itoa(int(cfg.Window.Seconds())))
			response.TooManyRequests(c, i18n.T(c.Request.Context(), cfg.MessageKey))
			c.Abort()
			return
		}
		c.Next()
	}
}
