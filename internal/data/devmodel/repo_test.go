package devmodel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
)

// TestDeleteModel_PlatformGoneIdempotent 平台侧型号已删（404）→ 幂等放行返回 nil
func TestDeleteModel_PlatformGoneIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "设备型号不存在"})
	}))
	defer srv.Close()

	a := NewGatewayAdapter(platformsdk.NewClient(srv.URL, "mk_test", "secret-test"))
	if err := a.DeleteModel(context.Background(), 404); err != nil {
		t.Fatalf("平台 404 应幂等放行, got %v", err)
	}
}
