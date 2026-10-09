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

// TestDeleteModel_NotFoundPassthrough 平台 404（不存在 / 通用或他商户型号不可写）→ 原样透传，
// 不再幂等放行（2026-10-08 商户归属收敛后 404 兼具「不可写」语义，静默放行会让未生效删除假装成功）
func TestDeleteModel_NotFoundPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "设备型号不存在"})
	}))
	defer srv.Close()

	a := NewGatewayAdapter(platformsdk.NewClient(srv.URL, "mk_test", "secret-test"))
	err := a.DeleteModel(context.Background(), 404)
	var se *platformsdk.Error
	if !errors.As(err, &se) || se.HTTPStatus != http.StatusNotFound {
		t.Fatalf("平台 404 应透传为 *platformsdk.Error, got %v", err)
	}
}
