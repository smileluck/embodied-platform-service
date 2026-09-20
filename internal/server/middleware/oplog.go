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
	"PUT /api/v1/auth/profile":              "更新个人信息",
	"PUT /api/v1/auth/password":             "修改密码",
	"PUT /api/v1/users/:id/admission":       "设置准入状态",
	"PUT /api/v1/users/:id/roles":           "分配角色",
	"POST /api/v1/users":                    "新增用户并同步平台",
	"DELETE /api/v1/users/:id":              "移除用户（解除平台关联）",
	"POST /api/v1/users/sync":               "从平台同步成员",
	"POST /api/v1/roles":                    "新增角色",
	"PUT /api/v1/roles/:id":                 "编辑角色",
	"DELETE /api/v1/roles/:id":              "删除角色",
	"PUT /api/v1/roles/:id/permissions":     "分配权限",
	"POST /api/v1/permissions":              "新增权限",
	"PUT /api/v1/permissions/:id":           "编辑权限",
	"DELETE /api/v1/permissions/:id":        "删除权限",
	"DELETE /api/v1/operation-logs":         "清理操作日志",
	"POST /api/v1/files":                    "上传文件",
	"DELETE /api/v1/files/:id":              "删除文件",
	"POST /api/v1/tenants":                  "新增租户",
	"PUT /api/v1/tenants/:id":               "编辑租户",
	"DELETE /api/v1/tenants/:id":            "删除租户",
	"PUT /api/v1/tenants/:id/status":        "修改租户状态",
	"POST /api/v1/tenants/:id/sync":         "同步租户至平台",
	"POST /api/v1/app-users":                "新增应用用户",
	"PUT /api/v1/app-users/:id":             "编辑应用用户",
	"DELETE /api/v1/app-users/:id":          "删除应用用户",
	"PUT /api/v1/app-users/:id/password":    "重置应用用户密码",
	"POST /api/v1/devices":                  "注册设备",
	"POST /api/v1/devices/:id/commands":     "下发设备命令",
	"POST /api/v1/device-models":            "新增设备型号",
	"PUT /api/v1/device-models/:id":         "编辑设备型号",
	"DELETE /api/v1/device-models/:id":      "删除设备型号",
	"POST /api/v1/ip-blacklist":             "新增IP黑名单",
	"DELETE /api/v1/ip-blacklist/:id":       "解封IP",
	"POST /api/v1/users/export":             "导出用户列表",
	"POST /api/v1/operation-logs/export":    "导出操作日志",
	"POST /api/v1/agent/providers":          "新增模型供应商",
	"PUT /api/v1/agent/providers/:id":       "编辑模型供应商",
	"DELETE /api/v1/agent/providers/:id":    "删除模型供应商",
	"POST /api/v1/agent/providers/:id/test": "测试供应商连通性",
	"POST /api/v1/agent/models":             "新增模型",
	"PUT /api/v1/agent/models/:id":          "编辑模型",
	"DELETE /api/v1/agent/models/:id":       "删除模型",
	"POST /api/v1/agent/models/:id/test":    "测试模型连通性",
	"POST /api/v1/agents":                   "新增 Agent",
	"PUT /api/v1/agents/:id":                "编辑 Agent",
	"DELETE /api/v1/agents/:id":             "删除 Agent",
	"POST /api/v1/agents/:id/chat":          "Agent 调试对话",
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
