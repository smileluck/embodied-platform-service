package devmodel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
)

// TestDeleteMapping_NotFoundPassthrough 平台 404（不存在 / 通用或他商户资源不可写）→ 原样透传，
// 不幂等放行（与 DeleteModel 同语义，静默放行会让未生效删除假装成功）
func TestDeleteMapping_NotFoundPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "数据映射不存在"})
	}))
	defer srv.Close()

	a := NewMappingGatewayAdapter(platformsdk.NewClient(srv.URL, "mk_test", "secret-test"))
	err := a.DeleteMapping(context.Background(), 404)
	var se *platformsdk.Error
	if !errors.As(err, &se) || se.HTTPStatus != http.StatusNotFound {
		t.Fatalf("平台 404 应透传为 *platformsdk.Error, got %v", err)
	}
}

// TestGetPublishedMapping_NoPublishedNil 型号无已发布映射版本：平台返回 data=null，
// 适配器须保持 nil, nil（这是正常态而非错误，前端据此展示「未配置」）
func TestGetPublishedMapping_NoPublishedNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open-api/v1/data-mappings/effective" || r.URL.Query().Get("model_id") != "7" {
			t.Errorf("unexpected request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": nil})
	}))
	defer srv.Close()

	a := NewMappingGatewayAdapter(platformsdk.NewClient(srv.URL, "mk_test", "secret-test"))
	v, err := a.GetPublishedMapping(context.Background(), 7)
	if err != nil {
		t.Fatalf("无已发布版本不应报错, got %v", err)
	}
	if v != nil {
		t.Fatalf("无已发布版本应返回 nil, got %+v", v)
	}
}

// TestGetPublishedMapping_Conversion 生效版本查询的字段转换（含嵌套 mappings 与 revert_of）
func TestGetPublishedMapping_Conversion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "msg": "ok",
			"data": map[string]any{
				"id": 11, "def_id": 3, "version": 2, "label": "1.1.0",
				"status": "published", "revert_of": 9, "published_at": "2026-10-09T10:00:00Z",
				"mappings": []map[string]any{{
					"source_topic": "/device/{sn}/joint_states", "topic_match": "exact",
					"data_category": "telemetry", "data_type": "joint_states",
					"fields": []string{"pos", "vel"},
				}},
				"created_at": "2026-10-09T09:00:00Z", "updated_at": "2026-10-09T10:00:00Z",
			},
		})
	}))
	defer srv.Close()

	a := NewMappingGatewayAdapter(platformsdk.NewClient(srv.URL, "mk_test", "secret-test"))
	v, err := a.GetPublishedMapping(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil || v.ID != 11 || v.DefID != 3 || v.Version != 2 || v.Label != "1.1.0" || v.Status != "published" {
		t.Fatalf("version fields mismatch: %+v", v)
	}
	if v.RevertOf == nil || *v.RevertOf != 9 {
		t.Fatalf("revert_of mismatch: %+v", v.RevertOf)
	}
	if len(v.Mappings) != 1 || v.Mappings[0].SourceTopic != "/device/{sn}/joint_states" ||
		v.Mappings[0].DataCategory != "telemetry" || len(v.Mappings[0].Fields) != 2 {
		t.Fatalf("mappings conversion mismatch: %+v", v.Mappings)
	}
}
