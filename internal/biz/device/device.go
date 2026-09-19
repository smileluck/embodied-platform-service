// Package device 设备限界上下文 —— 领域层。
//
// 纯代理：设备真源在 embodied-platform，本系统不落任何设备表。
// 经平台开放面（商户 HMAC，SDK 类型即对外契约）访问设备域；数据范围由平台
// 按商户绑定租户集服务端收敛（本系统不可指定 tenant_id 过滤）。
// 设备注册时的租户取自本地租户表（仅允许已同步至平台的租户）。
package device

import (
	"context"
	"errors"
	"net/http"

	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

// ErrTenantNotSynced 注册设备时所选租户未同步至平台（或不存在/被停用）
var ErrTenantNotSynced = errors.New("所选租户不可用：请选择已同步至平台且处于启用状态的租户")

// TenantReader 本地租户读取（跨上下文最小接口：设备注册校验用）
type TenantReader interface {
	// TenantAvailable 本地存在该平台租户 ID 对应的租户且已启用
	TenantAvailable(ctx context.Context, platformTenantID uint) (bool, error)
}

// TenantRelinker 平台侧租户丢失时按本地租户补链重建（由租户用例实现；
// LinkOrCreate 幂等——平台侧活租户按 code 命中即更新，不会凭空造出租户）
type TenantRelinker interface {
	RelinkByPlatformID(ctx context.Context, platformTenantID uint) (uint, error)
}

// Gateway 平台设备网关接口（data 层经开放面 SDK 实现，依赖倒置）
type Gateway interface {
	ListDevices(ctx context.Context, page, pageSize int, filter sdk.DeviceFilter) ([]*sdk.Device, *sdk.Page, error)
	GetDevice(ctx context.Context, id uint) (*sdk.Device, error)
	RegisterDevice(ctx context.Context, req sdk.RegisterDeviceRequest) (*sdk.Device, error)
	GetShadow(ctx context.Context, deviceID uint) (*sdk.Shadow, error)
	IssueCommand(ctx context.Context, deviceID uint, req sdk.IssueCommandRequest) (*sdk.Command, error)
	ListDeviceCommands(ctx context.Context, deviceID uint, status string, page, pageSize int) ([]*sdk.Command, *sdk.Page, error)
	GetCommand(ctx context.Context, id uint) (*sdk.Command, error)
	QueryTelemetry(ctx context.Context, deviceID uint, q sdk.TelemetryQuery) (*sdk.TelemetryHistory, error)
	ListDataEvents(ctx context.Context, deviceID, sinceID uint, limit int) ([]*sdk.DataEvent, error)
}

// Usecase 设备领域用例
type Usecase struct {
	gw       Gateway
	tenants  TenantReader
	relinker TenantRelinker
}

func NewUsecase(gw Gateway, tenants TenantReader, relinker TenantRelinker) *Usecase {
	return &Usecase{gw: gw, tenants: tenants, relinker: relinker}
}

func (uc *Usecase) List(ctx context.Context, page, pageSize int, filter sdk.DeviceFilter) ([]*sdk.Device, *sdk.Page, error) {
	return uc.gw.ListDevices(ctx, page, pageSize, filter)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*sdk.Device, error) {
	return uc.gw.GetDevice(ctx, id)
}

// Register 注册设备：租户必须来自本地租户表且已同步（platform_id 非零）并启用；
// 通过后经开放面注册（平台还会校验租户在商户绑定集内）。
// 平台 403（本地租户镜像悬挂——平台租户被删导致不在绑定集）时补链重建一次再重试；
// LinkOrCreate 幂等保证 scope 拒绝等其他 403 场景重试无害（重试仍失败则透传原错误）
func (uc *Usecase) Register(ctx context.Context, req sdk.RegisterDeviceRequest) (*sdk.Device, error) {
	ok, err := uc.tenants.TenantAvailable(ctx, req.TenantID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrTenantNotSynced
	}
	dev, err := uc.gw.RegisterDevice(ctx, req)
	if err == nil || uc.relinker == nil || !isForbidden(err) {
		return dev, err
	}
	newPID, rerr := uc.relinker.RelinkByPlatformID(ctx, req.TenantID)
	if rerr != nil {
		return nil, err // 补链失败：原错误透传
	}
	req.TenantID = newPID
	return uc.gw.RegisterDevice(ctx, req)
}

// isForbidden 平台开放面 403（租户不在商户绑定集 / scope 拒绝）
func isForbidden(err error) bool {
	var se *sdk.Error
	return errors.As(err, &se) && se.HTTPStatus == http.StatusForbidden
}

func (uc *Usecase) Shadow(ctx context.Context, deviceID uint) (*sdk.Shadow, error) {
	return uc.gw.GetShadow(ctx, deviceID)
}

// IssueCommand 下发命令（priority 语义：0 急停最高危；caller 由平台固定为本服务商户）
func (uc *Usecase) IssueCommand(ctx context.Context, deviceID uint, req sdk.IssueCommandRequest) (*sdk.Command, error) {
	return uc.gw.IssueCommand(ctx, deviceID, req)
}

func (uc *Usecase) ListCommands(ctx context.Context, deviceID uint, status string, page, pageSize int) ([]*sdk.Command, *sdk.Page, error) {
	return uc.gw.ListDeviceCommands(ctx, deviceID, status, page, pageSize)
}

func (uc *Usecase) GetCommand(ctx context.Context, id uint) (*sdk.Command, error) {
	return uc.gw.GetCommand(ctx, id)
}

func (uc *Usecase) Telemetry(ctx context.Context, deviceID uint, q sdk.TelemetryQuery) (*sdk.TelemetryHistory, error) {
	return uc.gw.QueryTelemetry(ctx, deviceID, q)
}

func (uc *Usecase) DataEvents(ctx context.Context, deviceID, sinceID uint, limit int) ([]*sdk.DataEvent, error) {
	return uc.gw.ListDataEvents(ctx, deviceID, sinceID, limit)
}
