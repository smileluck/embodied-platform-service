package devmodel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	bizdevmodel "github.com/smilex/smilex-admin-gin/internal/biz/devmodel"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
)

// TestDeleteTMNode_NotFoundPassthrough 平台 404（不存在 / 通用或他商户节点不可写）→ 原样透传，
// 不幂等放行（与 DeleteModel/DeleteMapping 同语义）
func TestDeleteTMNode_NotFoundPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "物模型节点不存在"})
	}))
	defer srv.Close()

	a := NewGatewayAdapter(platformsdk.NewClient(srv.URL, "mk_test", "secret-test"))
	err := a.DeleteTMNode(context.Background(), 404)
	var se *platformsdk.Error
	if !errors.As(err, &se) || se.HTTPStatus != http.StatusNotFound {
		t.Fatalf("平台 404 应透传为 *platformsdk.Error, got %v", err)
	}
}

// TestUpdateTMDraft_SchemaRoundTrip 草稿 schema 经 biz→SDK 转换后按原结构发出（要素级字段不失真）
func TestUpdateTMDraft_SchemaRoundTrip(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/open-api/v1/thing-models/versions/9" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok"})
	}))
	defer srv.Close()

	minV := 0.0
	a := NewGatewayAdapter(platformsdk.NewClient(srv.URL, "mk_test", "secret-test"))
	err := a.UpdateTMDraft(context.Background(), 9, &bizdevmodel.TMSchema{
		Properties: map[string]bizdevmodel.TMProperty{
			"temp": {DataType: "float", Unit: "℃", Writable: false, Min: &minV, Description: "温度"},
		},
		Services: map[string]bizdevmodel.TMService{
			"reboot": {CallType: "sync", Params: map[string]bizdevmodel.TMParam{
				"delay": {DataType: "int", Required: true},
			}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	schema, ok := gotBody["schema"].(map[string]any)
	if !ok {
		t.Fatalf("body 应含 schema 对象, got %v", gotBody)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok || props["temp"].(map[string]any)["unit"] != "℃" {
		t.Fatalf("properties 转换失真: %v", schema)
	}
	if props["temp"].(map[string]any)["min"].(float64) != 0 {
		t.Fatalf("min 指针字段应保留零值, got %v", props["temp"])
	}
	svcs, ok := schema["services"].(map[string]any)
	if !ok || svcs["reboot"].(map[string]any)["call_type"] != "sync" {
		t.Fatalf("services 转换失真: %v", schema)
	}
}

// TestResolveTM_Conversion 合并解析结果转换（schema + 继承链节点/版本）
func TestResolveTM_Conversion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open-api/v1/thing-models/3/resolve" || r.URL.Query().Get("version_id") != "7" {
			t.Errorf("unexpected request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"schema": map[string]any{
					"properties": map[string]any{
						"temp": map[string]any{"data_type": "float", "writable": false},
					},
				},
				"chain": []map[string]any{{
					"node":    map[string]any{"id": 1, "layer": "base", "code": "base", "name": "基础", "status": 1, "merchant_id": 0},
					"version": map[string]any{"id": 5, "node_id": 1, "version": 2, "status": "published", "published_at": "2026-10-09T10:00:00Z"},
				}},
			},
		})
	}))
	defer srv.Close()

	a := NewGatewayAdapter(platformsdk.NewClient(srv.URL, "mk_test", "secret-test"))
	res, err := a.ResolveTM(context.Background(), 3, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Schema == nil || len(res.Schema.Properties) != 1 {
		t.Fatalf("schema 转换失败: %+v", res)
	}
	if len(res.Chain) != 1 || res.Chain[0].Node.ID != 1 || res.Chain[0].Version.Version != 2 {
		t.Fatalf("chain 转换失败: %+v", res.Chain)
	}
}
