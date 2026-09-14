// 操作日志中间件：自动审计全部写请求（POST/PUT/DELETE/PATCH）。
// 挂载在 JWT 之后（取操作人）、RBAC 之前（权限拒绝的尝试也记录）；
// 401（token 无效/过期）在 JWT 处 abort，天然不产生记录。
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
)

// OpLogRecorder 操作日志记录入口（由 log 应用服务实现，异步落库）
type OpLogRecorder interface {
	RecordOperation(ctx context.Context, o *bizlog.OperationLog)
}

const (
	// opLogBodyLimit 参数摘要最多读取的 body 字节数
	opLogBodyLimit = 4 * 1024
	// opLogParamsMax 参数摘要落库最大长度
	opLogParamsMax = 2000
)

// actionNames 路由模板 -> 中文动作名（"METHOD 路由模板" 为键，未命中回退「METHOD 路由」）
var actionNames = map[string]string{
	"POST /api/v1/auth/logout":              "退出登录",
	"PUT /api/v1/auth/profile":              "更新个人信息",
	"PUT /api/v1/auth/password":             "修改密码",
	"POST /api/v1/users":                    "新增用户",
	"PUT /api/v1/users/:id":                 "编辑用户",
	"DELETE /api/v1/users/:id":              "删除用户",
	"PUT /api/v1/users/:id/roles":           "分配角色",
	"PUT /api/v1/users/:id/password":        "重置密码",
	"DELETE /api/v1/users/:id/sessions":     "踢用户下线",
	"POST /api/v1/roles":                    "新增角色",
	"PUT /api/v1/roles/:id":                 "编辑角色",
	"DELETE /api/v1/roles/:id":              "删除角色",
	"PUT /api/v1/roles/:id/permissions":     "分配权限",
	"POST /api/v1/permissions":              "新增权限",
	"PUT /api/v1/permissions/:id":           "编辑权限",
	"DELETE /api/v1/permissions/:id":        "删除权限",
	"DELETE /api/v1/online-users/:sid":      "下线会话",
	"DELETE /api/v1/login-logs":             "清理登录日志",
	"DELETE /api/v1/operation-logs":         "清理操作日志",
	"POST /api/v1/files":                    "上传文件",
	"DELETE /api/v1/files/:id":              "删除文件",
	"POST /api/v1/merchants":                "新增商户",
	"PUT /api/v1/merchants/:id":             "编辑商户",
	"DELETE /api/v1/merchants/:id":          "删除商户",
	"PUT /api/v1/merchants/:id/secret":      "重置商户密钥",
	"PUT /api/v1/merchants/:id/status":      "修改商户状态",
}

// OpLog 写请求自动审计
func OpLog(rec OpLogRecorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		default:
			c.Next()
			return
		}
		start := time.Now()
		body := peekBody(c)
		c.Next()

		o := &bizlog.OperationLog{
			Method:     c.Request.Method,
			Path:       requestPath(c),
			Route:      c.FullPath(),
			Action:     actionName(c.Request.Method, c.FullPath()),
			Params:     maskParams(c, body),
			IP:         c.ClientIP(),
			UserAgent:  truncateStr(c.GetHeader("User-Agent"), 255),
			StatusCode: c.Writer.Status(),
			LatencyMs:  int(time.Since(start).Milliseconds()),
			CreatedAt:  start,
		}
		if sub := Subject(c); sub != nil {
			o.UserID, o.Username = sub.UserID, sub.Username
		}
		rec.RecordOperation(context.Background(), o)
	}
}

// actionName 查中文动作名，未命中回退「METHOD 路由模板」
func actionName(method, route string) string {
	if name, ok := actionNames[method+" "+route]; ok {
		return name
	}
	if route == "" {
		return method
	}
	return method + " " + route
}

// requestPath 实际请求路径（含 query，截断到字段长度）
func requestPath(c *gin.Context) string {
	p := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		p += "?" + c.Request.URL.RawQuery
	}
	return truncateStr(p, 255)
}

// peekBody 预读 body 前 opLogBodyLimit 字节并回填（不影响后续 handler 绑定）
func peekBody(c *gin.Context) []byte {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(c.Request.Body, opLogBodyLimit))
	if err != nil {
		return nil
	}
	c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(b), c.Request.Body))
	return b
}

// maskParams 生成脱敏后的参数摘要：password/secret/token 类字段打码，
// 无 body 时记录 query；改密接口整体打码；超长截断。
func maskParams(c *gin.Context, body []byte) string {
	// 修改密码接口 body 全是凭据，整体脱敏
	if c.FullPath() == "/api/v1/auth/password" {
		return "（已脱敏）"
	}
	// multipart 上传 body 是二进制内容，不入参数摘要
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/") {
		return "（multipart 表单）"
	}
	if len(body) > 0 {
		var payload interface{}
		if json.Unmarshal(body, &payload) == nil {
			if cleaned, err := json.Marshal(maskValue(payload)); err == nil {
				body = cleaned
			}
		}
	}
	s := strings.TrimSpace(string(body))
	if s == "" {
		s = c.Request.URL.RawQuery
	}
	if len(s) > opLogParamsMax {
		s = s[:opLogParamsMax] + "…（截断）"
	}
	return s
}

// maskValue 递归打码敏感字段（与 security.sanitizeJSON 的 password 跳过逻辑同源）
func maskValue(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, val := range x {
			lk := strings.ToLower(k)
			if strings.Contains(lk, "password") || strings.Contains(lk, "secret") || strings.Contains(lk, "token") {
				x[k] = "***"
				continue
			}
			x[k] = maskValue(val)
		}
		return x
	case []interface{}:
		for i, val := range x {
			x[i] = maskValue(val)
		}
		return x
	default:
		return v
	}
}

// truncateStr 按字节长度截断（防超长存储）
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
