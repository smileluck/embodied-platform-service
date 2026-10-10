// 安全类中间件：安全响应头、XSS 输入清洗、SQL 注入特征拦截。
// 登录/通用限流见 ratelimit.go。
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"github.com/smilex/smilex-admin-gin/pkg/security"
)

// SecurityHeaders 安全响应头：防 MIME 嗅探、点击劫持与协议泄露。
// CSP 面向 SPA 托管（web/dist）：style unsafe-inline 兼容 naive-ui 的 cssinjs，
// img data: 兼容 base64 验证码、https: 兼容菜单网络图标。
// HSTS 仅在 TLS（含反代 X-Forwarded-Proto 标记）下发送——明文 HTTP 下浏览器会忽略该头，
// 且中间人可剥离，发了也无防护意义。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; "+
				"script-src 'self'; connect-src 'self'; font-src 'self' data:; "+
				"frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		c.Next()
	}
}

// XSSFilter 对 JSON body 的字符串值剥离 HTML 并去首尾空格（存储型 XSS 纵深防御 +
// 服务端兜底 trim，与前端 deepTrim 同口径，防止绕过前端校验写入脏数据）。
// 仅处理 application/json 的写入类请求；解析失败原样放行，由后续 ShouldBindJSON 报错。
func XSSFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
			if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
				if body, err := io.ReadAll(c.Request.Body); err == nil {
					c.Request.Body = io.NopCloser(bytes.NewReader(sanitizeJSON(body)))
				}
			}
		}
		c.Next()
	}
}

// sanitizeJSON 递归清洗 JSON 文档中的字符串值；非法 JSON 原样返回。
// 密码类字段（key 含 password）跳过 —— 合法密码允许包含尖括号，且静默改写密码
// 会让用户实际输入与落库值不一致。
func sanitizeJSON(body []byte) []byte {
	var payload interface{}
	if json.Unmarshal(body, &payload) != nil {
		return body
	}
	if _, ok := payload.(map[string]interface{}); !ok {
		return body
	}
	cleaned, err := json.Marshal(sanitizeValue(payload))
	if err != nil {
		return body
	}
	return cleaned
}

func sanitizeValue(v interface{}) interface{} {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(security.StripHTML(x))
	case map[string]interface{}:
		for k, val := range x {
			if strings.Contains(strings.ToLower(k), "password") {
				continue
			}
			x[k] = sanitizeValue(val)
		}
		return x
	case []interface{}:
		for i, val := range x {
			x[i] = sanitizeValue(val)
		}
		return x
	default:
		return v
	}
}

// 查询入参防线共用上限：单个参数值最大长度（rune 计）。搜索关键字等文本入参
// 远用不到该长度，超长值只可能来自滥用探测。
const maxQueryValueLen = 256

// SQLInjectionGuard 查询入参 WAF 式前置防线（入口层纵深防御，数据层本就全参数化）：
//   - 值先统一去首尾空格（与前端 deepTrim 同口径，防绕过前端写入带空格的脏查询）
//   - 高危 SQL 注入特征 / XSS 载体特征 → 400 拦截
//   - 单值超过 maxQueryValueLen → 400（防超长入参直达查询层）
//   - page / page_size 类型与取值校验：非整数、page<1、page_size<0 → 400
//     （page_size=0 为本系统「全量」约定值，放行；上限夹取在 pageParams）
func SQLInjectionGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Request.URL.Query()
		trimmed := false
		for key, vs := range q {
			for i, v := range vs {
				t := strings.TrimSpace(v)
				if t != v {
					vs[i] = t
					trimmed = true
				}
				if utf8.RuneCountInString(t) > maxQueryValueLen {
					response.BadRequest(c, i18n.T(c.Request.Context(), "security.param_too_long"))
					c.Abort()
					return
				}
				if security.ContainsSQLInjection(t) || security.ContainsXSSPayload(t) {
					response.BadRequest(c, i18n.T(c.Request.Context(), "security.invalid_chars"))
					c.Abort()
					return
				}
			}
			if key == "page" || key == "page_size" {
				for _, v := range vs {
					if n, err := strconv.Atoi(v); err != nil ||
						(key == "page" && n < 1) || (key == "page_size" && n < 0) {
						response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
						c.Abort()
						return
					}
				}
			}
		}
		if trimmed {
			c.Request.URL.RawQuery = q.Encode()
		}
		for _, p := range c.Params {
			if security.ContainsSQLInjection(p.Value) {
				response.BadRequest(c, i18n.T(c.Request.Context(), "security.invalid_chars"))
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
