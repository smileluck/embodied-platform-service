package platformsdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeOpenAPIServer 服务端视角对账：验签串、信封解包、错误透出
func fakeOpenAPIServer(t *testing.T, pathHit *string, resp string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*pathHit = r.URL.RequestURI() // 含 query，供断言
		// 服务端按平台口径重算签名（含 body hash）
		if r.Header.Get(HeaderAppKey) == "" || r.Header.Get(HeaderSign) == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 401, "msg": "missing sign headers"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(resp))
	}))
}

func TestClient_PingEnvelope(t *testing.T) {
	var path string
	srv := fakeOpenAPIServer(t, &path, `{"code":0,"msg":"ok","data":{"app_key":"mk_x"}}`, 200)
	defer srv.Close()

	c := NewClient(srv.URL, "mk_x", "s")
	out, err := c.Ping(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if path != "/open-api/v1/ping" || out["app_key"] != "mk_x" {
		t.Errorf("ping: path=%s out=%v", path, out)
	}
}

func TestClient_ErrorEnvelope(t *testing.T) {
	var path string
	srv := fakeOpenAPIServer(t, &path, `{"code":403,"msg":"Merchant is not authorized to access this API","data":null}`, 403)
	defer srv.Close()

	c := NewClient(srv.URL, "mk_x", "s")
	_, err := c.GetDevice(context.Background(), 1)
	apiErr, ok := err.(*Error)
	if !ok || apiErr.HTTPStatus != 403 || apiErr.Code != 403 {
		t.Fatalf("应透出 *Error, got %#v", err)
	}
	if !strings.Contains(apiErr.Error(), "not authorized") {
		t.Errorf("错误信息应含服务端 msg: %v", apiErr)
	}
}

func TestClient_GwPrefixPathSigning(t *testing.T) {
	var path string
	srv := fakeOpenAPIServer(t, &path, `{"code":0,"msg":"ok","data":{}}`, 200)
	defer srv.Close()

	// /gw 形态：BaseURL 携带网关前缀，请求 path 应为 /gw/open-api/v1/...（签名同口径）
	c := NewClient(srv.URL+"/gw/open-api", "mk_x", "s")
	if _, err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if path != "/gw/open-api/v1/ping" {
		t.Errorf("gw 形态 path=%s, want /gw/open-api/v1/ping", path)
	}
}

func TestClient_ListPagination(t *testing.T) {
	var path string
	srv := fakeOpenAPIServer(t, &path, `{"code":0,"msg":"ok","data":{"list":[{"id":1,"sn":"sn1"}],"page":{"page":1,"page_size":20,"total":1}}}`, 200)
	defer srv.Close()

	c := NewClient(srv.URL, "mk_x", "s")
	list, pg, err := c.ListDevices(context.Background(), 1, 20, DeviceFilter{Keyword: "sn"})
	if err != nil || len(list) != 1 || pg.Total != 1 {
		t.Fatalf("list: %v %v %v", list, pg, err)
	}
	if !strings.Contains(path, "/devices?") || !strings.Contains(path, "kw=sn") {
		t.Errorf("query 拼装异常: %s", path)
	}
}
