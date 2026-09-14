// 开放 API 商户验签中间件：HMAC-SHA256 签名 + 时间戳偏差 + nonce 防重放。
// 挂载在 /open-api/v1 组上（IP 黑名单之后）；验签通过将商户注入请求上下文，
// 全部调用（成功/失败）均异步落 merchant_api_logs（不记 body，Msg 只记失败原因摘要）。
package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	bizmerchant "github.com/smilex/smilex-admin-gin/internal/biz/merchant"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

const ctxMerchantKey = "openapi.merchant"

// 签名请求头
const (
	headerAppKey    = "X-App-Key"
	headerTimestamp = "X-Timestamp"
	headerNonce     = "X-Nonce"
	headerSign      = "X-Sign"
)

// apiLogMsgMax 调用日志失败原因摘要限长
const apiLogMsgMax = 255

// MerchantFromContext 从 context 取已验签商户（OpenAPIAuth 之后可用）
func MerchantFromContext(c *gin.Context) *bizmerchant.Merchant {
	if v, ok := c.Get(ctxMerchantKey); ok {
		if m, ok := v.(*bizmerchant.Merchant); ok {
			return m
		}
	}
	return nil
}

// OpenAPIAuth 开放 API 商户验签：
//   - 签名头缺失/时间戳超偏差/nonce 重放/验签失败均 401 并记调用日志；
//   - nonce 去重存 Redis（SETNX + TTL），Redis 故障 fail-closed（一律拒绝，仿 JWT 会话校验）；
//   - 验签通过：商户注入 context，handler 完成后按实际状态码与耗时异步落调用日志。
func OpenAPIAuth(uc *bizmerchant.Usecase, rdb *redis.Client, cfg conf.OpenAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		appKey := c.GetHeader(headerAppKey)
		timestamp := c.GetHeader(headerTimestamp)
		nonce := c.GetHeader(headerNonce)
		sign := c.GetHeader(headerSign)

		// deny 统一拒绝路径：记调用日志（商户未知时 MerchantID=0、AppKey 原样）后 401
		deny := func(msg string, m *bizmerchant.Merchant) {
			var merchantID uint
			if m != nil {
				merchantID = m.ID
			}
			recordAccess(c, uc, start, merchantID, appKey, http.StatusUnauthorized, msg)
			response.Unauthorized(c, msg)
			c.Abort()
		}

		if appKey == "" || timestamp == "" || nonce == "" || sign == "" {
			deny(i18n.T(c.Request.Context(), "openapi.missing_sign_headers"), nil)
			return
		}

		// 时间戳为 unix 秒，偏差超过 signSkewSeconds 即拒绝（防重放窗口）
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || skewExceeded(ts, cfg.SignSkewSeconds) {
			deny(i18n.T(c.Request.Context(), "openapi.invalid_timestamp"), nil)
			return
		}

		// nonce 长度限制 8-64（防超长键与无意义重放占用）
		if len(nonce) < 8 || len(nonce) > 64 {
			deny(i18n.T(c.Request.Context(), "openapi.invalid_nonce"), nil)
			return
		}
		ok, err := rdb.SetNX(c.Request.Context(), "openapi:nonce:"+appKey+":"+nonce, 1,
			time.Duration(cfg.NonceTTLSeconds)*time.Second).Result()
		if err != nil {
			// Redis 故障 fail-closed：防重放失效时宁可拒绝也不放行
			deny(i18n.T(c.Request.Context(), "openapi.service_unavailable"), nil)
			return
		}
		if !ok {
			deny(i18n.T(c.Request.Context(), "openapi.nonce_replayed"), nil)
			return
		}

		// body hash：读出后复位，供后续 handler 绑定使用（日志不记 body）
		var body []byte
		if c.Request.Body != nil {
			if body, err = io.ReadAll(c.Request.Body); err != nil {
				deny(i18n.T(c.Request.Context(), "common.invalid_params"), nil)
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		bodySum := sha256.Sum256(body)
		bodyHash := hex.EncodeToString(bodySum[:])

		m, err := uc.VerifySign(c.Request.Context(), appKey, c.Request.Method, c.Request.URL.Path, timestamp, nonce, bodyHash, sign)
		if err != nil {
			msg := err.Error()
			switch {
			case errors.Is(err, bizmerchant.ErrMerchantNotFound):
				msg = i18n.T(c.Request.Context(), "merchant.not_found")
			case errors.Is(err, bizmerchant.ErrMerchantDisabled):
				msg = i18n.T(c.Request.Context(), "merchant.disabled")
			case errors.Is(err, bizmerchant.ErrInvalidSign):
				msg = i18n.T(c.Request.Context(), "merchant.sign_invalid")
			}
			deny(msg, m)
			return
		}

		c.Set(ctxMerchantKey, m)
		c.Next()

		// 成功调用：handler 完成后按实际状态码与耗时落日志（异步）
		recordAccess(c, uc, start, m.ID, appKey, c.Writer.Status(), "")
	}
}

// skewExceeded 时间戳（unix 秒）与当前时间偏差是否超过允许值；skew<=0 时仅要求不晚于当前 60s
func skewExceeded(ts int64, skew int) bool {
	if skew <= 0 {
		skew = 60
	}
	diff := time.Now().Unix() - ts
	if diff < 0 {
		diff = -diff
	}
	return diff > int64(skew)
}

// recordAccess 异步落一条调用日志（失败原因摘要限长；脱离请求生命周期用 Background）
func recordAccess(c *gin.Context, uc *bizmerchant.Usecase, start time.Time, merchantID uint, appKey string, statusCode int, msg string) {
	uc.RecordAccess(context.Background(), &bizmerchant.APILog{
		MerchantID: merchantID,
		AppKey:     truncateStr(appKey, 64),
		Method:     c.Request.Method,
		Path:       truncateStr(c.Request.URL.Path, 255),
		IP:         c.ClientIP(),
		StatusCode: statusCode,
		LatencyMs:  int(time.Since(start).Milliseconds()),
		Msg:        truncateStr(msg, apiLogMsgMax),
		CreatedAt:  start,
	})
}
