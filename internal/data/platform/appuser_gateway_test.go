package platform

import (
	"errors"
	"net/http"
	"testing"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
)

// TestAppUserGatewayErrMapping 开放面错误 → 业务哨兵映射（409 按平台文案区分重名/跨商户）
func TestAppUserGatewayErrMapping(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"404 不存在", &platformsdk.Error{HTTPStatus: http.StatusNotFound}, bizappuser.ErrAppUserNotFound},
		{"403 租户越界", &platformsdk.Error{HTTPStatus: http.StatusForbidden}, bizappuser.ErrTenantNotInScope},
		{"409 重名", &platformsdk.Error{HTTPStatus: http.StatusConflict, Msg: "用户名已存在，请更换"}, bizappuser.ErrDuplicateUsername},
		{"409 跨商户删除", &platformsdk.Error{HTTPStatus: http.StatusConflict, Msg: "应用用户仍挂靠其他商户的租户，请先解除其在本商户外的归属"}, bizappuser.ErrCrossMerchantDelete},
		{"其他透传", &platformsdk.Error{HTTPStatus: http.StatusBadRequest, Msg: "x"}, nil},
	}
	for _, tc := range cases {
		got := mapAppUserErr(tc.in)
		if tc.want == nil {
			if !errors.Is(got, tc.in) {
				t.Errorf("%s: 应透传原错误, got %v", tc.name, got)
			}
			continue
		}
		if !errors.Is(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if mapAppUserErr(nil) != nil {
		t.Fatal("nil 输入应返回 nil")
	}
	// 非 sdk 错误（网络层等）原样透传，供上层按 503 语义处理
	plain := errors.New("boom")
	if got := mapAppUserErr(plain); !errors.Is(got, plain) {
		t.Errorf("非 sdk 错误应透传, got %v", got)
	}
}
