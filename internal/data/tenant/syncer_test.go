package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

// fakePlatformOpenAPI 假平台开放面（租户域）：校验 HMAC 签名头存在即可，
// 行为按用例脚本化（create 冲突/成功、findByCode 命中/未命中、/tenants/:id 类 404/500）。
type fakePlatformOpenAPI struct {
	t        *testing.T
	existing []*sdk.Tenant // 已有租户（列表返回）
	created  *sdk.Tenant   // 记录创建请求
	failNext int           // 接下来 N 个写请求返回 500
	gone     bool          // /tenants/:id 类请求恒返回 404（平台侧已删）
}

func (f *fakePlatformOpenAPI) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/open-api/v1/tenants", func(w http.ResponseWriter, r *http.Request) {
		for _, h := range []string{"X-App-Key", "X-Timestamp", "X-Nonce", "X-Sign"} {
			if r.Header.Get(h) == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 401, "msg": "missing " + h})
				return
			}
		}
		switch r.Method {
		case http.MethodPost:
			if f.failNext > 0 {
				f.failNext--
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "boom"})
				return
			}
			var req sdk.TenantCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			for _, e := range f.existing {
				if e.Code == req.Code {
					w.WriteHeader(http.StatusConflict)
					_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "租户编码已存在，请更换"})
					return
				}
			}
			vo := &sdk.Tenant{ID: 77, Name: req.Name, Code: req.Code, Status: 1}
			f.existing = append(f.existing, vo)
			f.created = vo
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": vo})
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": map[string]any{
				"list": f.existing,
				"page": map[string]any{"page": 1, "page_size": 50, "total": len(f.existing)},
			}})
		}
	})
	mux.HandleFunc("/open-api/v1/tenants/", func(w http.ResponseWriter, r *http.Request) {
		if f.gone {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "租户不存在"})
			return
		}
		if f.failNext > 0 {
			f.failNext--
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "boom"})
			return
		}
		switch r.Method {
		case http.MethodPut: // update / status：成功即可
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok"})
		case http.MethodDelete:
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok"})
		}
	})
	return mux
}

func newTestSyncer(f *fakePlatformOpenAPI) (*Syncer, *httptest.Server) {
	srv := httptest.NewServer(f.handler())
	return NewSyncer(sdk.NewClient(srv.URL, "mk_test", "secret-test")), srv
}

// TestSyncer_CreateOnPlatform 创建：走开放面（签名头齐备），返回平台 ID
func TestSyncer_CreateOnPlatform(t *testing.T) {
	f := &fakePlatformOpenAPI{t: t}
	syncer, srv := newTestSyncer(f)
	defer srv.Close()

	id, err := syncer.CreateOnPlatform(context.Background(), &biztenant.Tenant{
		Name: "租户A", Code: "tn-a", ContactName: "张三",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != 77 || f.created == nil || f.created.Code != "tn-a" {
		t.Fatalf("创建结果异常: id=%d created=%+v", id, f.created)
	}
}

// TestSyncer_CreateDuplicateCode 平台侧 code 冲突透传 *sdk.Error（HTTPStatus=409）
func TestSyncer_CreateDuplicateCode(t *testing.T) {
	f := &fakePlatformOpenAPI{t: t, existing: []*sdk.Tenant{{ID: 1, Code: "dup"}}}
	syncer, srv := newTestSyncer(f)
	defer srv.Close()

	_, err := syncer.CreateOnPlatform(context.Background(), &biztenant.Tenant{Name: "n", Code: "dup"})
	var se *sdk.Error
	if err == nil {
		t.Fatal("应返回冲突错误")
	}
	if ok := asSDKError(err, &se); !ok || se.HTTPStatus != http.StatusConflict {
		t.Fatalf("应为 *sdk.Error(409), got %v", err)
	}
}

// TestSyncer_LinkOrCreate 命中已有同 code 租户（本商户可见）→ 复用其 ID
func TestSyncer_LinkOrCreate(t *testing.T) {
	f := &fakePlatformOpenAPI{t: t, existing: []*sdk.Tenant{{ID: 55, Code: "exist-1", Name: "旧名"}}}
	syncer, srv := newTestSyncer(f)
	defer srv.Close()

	id, err := syncer.LinkOrCreateOnPlatform(context.Background(), &biztenant.Tenant{
		Name: "新名", Code: "exist-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != 55 {
		t.Fatalf("应复用已有租户 55, got %d", id)
	}
	if f.created != nil {
		t.Fatal("命中已有租户时不应创建")
	}
}

// TestSyncer_LinkOrCreate 未命中 → 创建
func TestSyncer_LinkOrCreateMissing(t *testing.T) {
	f := &fakePlatformOpenAPI{t: t}
	syncer, srv := newTestSyncer(f)
	defer srv.Close()

	id, err := syncer.LinkOrCreateOnPlatform(context.Background(), &biztenant.Tenant{Name: "n", Code: "fresh"})
	if err != nil || id != 77 {
		t.Fatalf("应创建新租户(77): id=%d err=%v", id, err)
	}
}

// TestSyncer_UpdateDeleteStatus 写路径失败透传
func TestSyncer_UpdateDeleteStatus(t *testing.T) {
	f := &fakePlatformOpenAPI{t: t}
	syncer, srv := newTestSyncer(f)
	defer srv.Close()
	ctx := context.Background()

	if err := syncer.UpdateOnPlatform(ctx, &biztenant.Tenant{PlatformID: 9, Name: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := syncer.SetStatusOnPlatform(ctx, 9, false); err != nil {
		t.Fatal(err)
	}
	if err := syncer.DeleteFromPlatform(ctx, 9); err != nil {
		t.Fatal(err)
	}

	f.failNext = 1
	if err := syncer.DeleteFromPlatform(ctx, 9); err == nil {
		t.Fatal("平台故障应透传错误")
	}
}

// TestSyncer_PlatformGoneMapping 平台侧 404（开放面对不存在/越权统一 404）→
// ErrPlatformTenantGone（更新/删除/启停）；非 404 错误原样透传
func TestSyncer_PlatformGoneMapping(t *testing.T) {
	f := &fakePlatformOpenAPI{t: t, gone: true}
	syncer, srv := newTestSyncer(f)
	defer srv.Close()
	ctx := context.Background()

	if err := syncer.UpdateOnPlatform(ctx, &biztenant.Tenant{PlatformID: 5, Name: "n"}); !errors.Is(err, biztenant.ErrPlatformTenantGone) {
		t.Fatalf("update 404 应映射 ErrPlatformTenantGone, got %v", err)
	}
	if err := syncer.DeleteFromPlatform(ctx, 5); !errors.Is(err, biztenant.ErrPlatformTenantGone) {
		t.Fatalf("delete 404 应映射 ErrPlatformTenantGone, got %v", err)
	}
	if err := syncer.SetStatusOnPlatform(ctx, 5, false); !errors.Is(err, biztenant.ErrPlatformTenantGone) {
		t.Fatalf("status 404 应映射 ErrPlatformTenantGone, got %v", err)
	}

	f2 := &fakePlatformOpenAPI{t: t, failNext: 1}
	syncer2, srv2 := newTestSyncer(f2)
	defer srv2.Close()
	if err := syncer2.DeleteFromPlatform(ctx, 5); err == nil || errors.Is(err, biztenant.ErrPlatformTenantGone) {
		t.Fatalf("非 404 错误应原样透传, got %v", err)
	}
}

func asSDKError(err error, target **sdk.Error) bool {
	if e, ok := err.(*sdk.Error); ok {
		*target = e
		return true
	}
	return false
}
