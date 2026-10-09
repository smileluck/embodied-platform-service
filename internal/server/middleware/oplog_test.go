// 操作日志中间件测试：TenantOpLog（tid/操作人提取、写方法过滤、凭据脱敏、动作名）。
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
)

// fakeTenantOpLogRecorder 租户门户操作日志记录替身（同步收集，便于断言）
type fakeTenantOpLogRecorder struct {
	logs []*bizlog.TenantOperationLog
}

func (f *fakeTenantOpLogRecorder) RecordTenantOperation(ctx context.Context, o *bizlog.TenantOperationLog) {
	f.logs = append(f.logs, o)
}

// injectTenantAuth 模拟 TenantAuth 之后的上下文注入
func injectTenantAuth(sub *bizauth.TenantSubject, tn *biztenant.Tenant) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ctxTenantAuthSubjectKey, sub)
		c.Set(ctxTenantAuthTenantKey, tn)
		c.Next()
	}
}

func doOp(r http.Handler, method, target, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(w, req)
	return w
}

func TestTenantOpLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := &fakeTenantOpLogRecorder{}
	e := gin.New()
	e.Use(injectTenantAuth(
		&bizauth.TenantSubject{UserID: 42, Username: "portal-admin", TenantID: 101},
		&biztenant.Tenant{ID: 1, PlatformID: 101, Name: "T1", Code: "t1", Status: 1},
	))
	e.Use(TenantOpLog(rec))
	e.GET("/tenant-api/v1/members", okHandler)
	e.POST("/tenant-api/v1/members", okHandler)
	e.PUT("/tenant-api/v1/profile/password", okHandler)
	e.PUT("/tenant-api/v1/members/:id/password", okHandler)

	// 读请求不落审计
	doOp(e, http.MethodGet, "/tenant-api/v1/members", "")
	if len(rec.logs) != 0 {
		t.Fatalf("GET 不应落审计，got %d 条", len(rec.logs))
	}

	// 写请求落审计：tid/操作人取自 TenantAuth 上下文，动作名命中注册表
	doOp(e, http.MethodPost, "/tenant-api/v1/members", `{"username":"bob","password":"p@ss123"}`)
	if len(rec.logs) != 1 {
		t.Fatalf("POST 应落 1 条审计，got %d", len(rec.logs))
	}
	o := rec.logs[0]
	if o.TenantID != 101 || o.UserID != 42 || o.Username != "portal-admin" {
		t.Fatalf("租户/操作人提取异常: tenant_id=%d user_id=%d username=%q", o.TenantID, o.UserID, o.Username)
	}
	if o.Action != "新增成员" {
		t.Fatalf("action = %q, want 新增成员", o.Action)
	}
	if o.Route != "/tenant-api/v1/members" || o.Method != http.MethodPost {
		t.Fatalf("route/method 异常: %s %s", o.Method, o.Route)
	}
	// 凭据字段递归打码
	if strings.Contains(o.Params, "p@ss123") {
		t.Fatalf("params 未脱敏: %s", o.Params)
	}
	if !strings.Contains(o.Params, "***") {
		t.Fatalf("params 缺少打码标记: %s", o.Params)
	}

	// 本人改密：body 整体脱敏
	doOp(e, http.MethodPut, "/tenant-api/v1/profile/password", `{"old_password":"a123456","new_password":"b123456"}`)
	if got := rec.logs[1].Params; got != "（已脱敏）" {
		t.Fatalf("改密 params = %q, want 整体脱敏", got)
	}

	// 重置成员密码：body 整体脱敏 + 动作名命中
	doOp(e, http.MethodPut, "/tenant-api/v1/members/7/password", `{"password":"c123456"}`)
	if got := rec.logs[2].Params; got != "（已脱敏）" {
		t.Fatalf("重置密码 params = %q, want 整体脱敏", got)
	}
	if got := rec.logs[2].Action; got != "重置成员密码" {
		t.Fatalf("重置密码 action = %q, want 重置成员密码", got)
	}
}

func TestTenantOpLogWithoutAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := &fakeTenantOpLogRecorder{}
	e := gin.New()
	e.Use(TenantOpLog(rec))
	e.POST("/tenant-api/v1/members", okHandler)

	doOp(e, http.MethodPost, "/tenant-api/v1/members", `{}`)
	if len(rec.logs) != 1 {
		t.Fatalf("应落 1 条审计，got %d", len(rec.logs))
	}
	// 无 TenantAuth 上下文：tid/操作人零值落库（不 panic）
	if o := rec.logs[0]; o.TenantID != 0 || o.UserID != 0 || o.Username != "" {
		t.Fatalf("缺失认证上下文应零值落库: %+v", o)
	}
}
