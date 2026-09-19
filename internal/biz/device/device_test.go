package device

import (
	"context"
	"errors"
	"net/http"
	"testing"

	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

// fakeGateway 平台设备网关假实现：第一次 Register 返回 403（租户不在绑定集），重试成功
type fakeGateway struct {
	registerCalls []uint // 记录每次注册的 TenantID
	dev           *sdk.Device
	firstErr      error
}

func (f *fakeGateway) ListDevices(ctx context.Context, page, pageSize int, filter sdk.DeviceFilter) ([]*sdk.Device, *sdk.Page, error) {
	return nil, nil, nil
}
func (f *fakeGateway) GetDevice(ctx context.Context, id uint) (*sdk.Device, error) { return nil, nil }
func (f *fakeGateway) RegisterDevice(ctx context.Context, req sdk.RegisterDeviceRequest) (*sdk.Device, error) {
	f.registerCalls = append(f.registerCalls, req.TenantID)
	if len(f.registerCalls) == 1 && f.firstErr != nil {
		return nil, f.firstErr
	}
	return f.dev, nil
}
func (f *fakeGateway) GetShadow(ctx context.Context, deviceID uint) (*sdk.Shadow, error) {
	return nil, nil
}
func (f *fakeGateway) IssueCommand(ctx context.Context, deviceID uint, req sdk.IssueCommandRequest) (*sdk.Command, error) {
	return nil, nil
}
func (f *fakeGateway) ListDeviceCommands(ctx context.Context, deviceID uint, status string, page, pageSize int) ([]*sdk.Command, *sdk.Page, error) {
	return nil, nil, nil
}
func (f *fakeGateway) GetCommand(ctx context.Context, id uint) (*sdk.Command, error) { return nil, nil }
func (f *fakeGateway) QueryTelemetry(ctx context.Context, deviceID uint, q sdk.TelemetryQuery) (*sdk.TelemetryHistory, error) {
	return nil, nil
}
func (f *fakeGateway) ListDataEvents(ctx context.Context, deviceID, sinceID uint, limit int) ([]*sdk.DataEvent, error) {
	return nil, nil
}

type fakeTenants struct{ available map[uint]bool }

func (f *fakeTenants) TenantAvailable(_ context.Context, platformTenantID uint) (bool, error) {
	return f.available[platformTenantID], nil
}

type fakeRelinker struct {
	calledWith []uint
	newPID     uint
	err        error
}

func (f *fakeRelinker) RelinkByPlatformID(_ context.Context, platformTenantID uint) (uint, error) {
	f.calledWith = append(f.calledWith, platformTenantID)
	if f.err != nil {
		return 0, f.err
	}
	return f.newPID, nil
}

// TestRegister_RelinkRetryOn403 平台 403（本地租户镜像悬挂）→ 补链重建后带新平台租户 ID 重试成功
func TestRegister_RelinkRetryOn403(t *testing.T) {
	gw := &fakeGateway{firstErr: &sdk.Error{HTTPStatus: http.StatusForbidden, Msg: "租户不在商户绑定范围内"}, dev: &sdk.Device{ID: 7}}
	relinker := &fakeRelinker{newPID: 99}
	uc := NewUsecase(gw, &fakeTenants{available: map[uint]bool{55: true}}, relinker)

	dev, err := uc.Register(context.Background(), sdk.RegisterDeviceRequest{SN: "sn-1", TenantID: 55})
	if err != nil {
		t.Fatal(err)
	}
	if dev.ID != 7 {
		t.Fatalf("重试应成功返回设备, got %+v", dev)
	}
	if len(relinker.calledWith) != 1 || relinker.calledWith[0] != 55 {
		t.Fatalf("应按原平台租户 ID 补链: %v", relinker.calledWith)
	}
	if len(gw.registerCalls) != 2 || gw.registerCalls[1] != 99 {
		t.Fatalf("重试应使用补链后的新租户 ID: %v", gw.registerCalls)
	}
}

// TestRegister_Non403NoRelink 非 403 错误（如 409 SN 重复）不触发补链，原样透传
func TestRegister_Non403NoRelink(t *testing.T) {
	conflict := &sdk.Error{HTTPStatus: http.StatusConflict, Msg: "SN 已存在"}
	gw := &fakeGateway{firstErr: conflict}
	relinker := &fakeRelinker{newPID: 99}
	uc := NewUsecase(gw, &fakeTenants{available: map[uint]bool{55: true}}, relinker)

	_, err := uc.Register(context.Background(), sdk.RegisterDeviceRequest{SN: "sn-1", TenantID: 55})
	var se *sdk.Error
	if !errors.As(err, &se) || se.HTTPStatus != http.StatusConflict {
		t.Fatalf("应透传冲突错误, got %v", err)
	}
	if len(relinker.calledWith) != 0 {
		t.Fatal("非 403 不应触发补链")
	}
	if len(gw.registerCalls) != 1 {
		t.Fatal("不应重试注册")
	}
}

// TestRegister_RelinkFailPropagates 补链失败时透传原始 403（不吞错误）
func TestRegister_RelinkFailPropagates(t *testing.T) {
	gw := &fakeGateway{firstErr: &sdk.Error{HTTPStatus: http.StatusForbidden, Msg: "租户不在商户绑定范围内"}}
	uc := NewUsecase(gw, &fakeTenants{available: map[uint]bool{55: true}},
		&fakeRelinker{err: errors.New("平台不可达")})

	_, err := uc.Register(context.Background(), sdk.RegisterDeviceRequest{SN: "sn-1", TenantID: 55})
	var se *sdk.Error
	if !errors.As(err, &se) || se.HTTPStatus != http.StatusForbidden {
		t.Fatalf("应透传原始 403, got %v", err)
	}
}
