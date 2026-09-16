// Package device 设备网关适配器（平台开放面 SDK）
package device

import (
	"context"

	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

// GatewayAdapter 开放面 SDK → biz 设备网关
type GatewayAdapter struct {
	client *sdk.Client
}

// NewGatewayAdapter 构造（wire provider，绑定 bizdevice.Gateway）
func NewGatewayAdapter(client *sdk.Client) *GatewayAdapter {
	return &GatewayAdapter{client: client}
}

func (a *GatewayAdapter) ListDevices(ctx context.Context, page, pageSize int, filter sdk.DeviceFilter) ([]*sdk.Device, *sdk.Page, error) {
	return a.client.ListDevices(ctx, page, pageSize, filter)
}

func (a *GatewayAdapter) GetDevice(ctx context.Context, id uint) (*sdk.Device, error) {
	return a.client.GetDevice(ctx, id)
}

func (a *GatewayAdapter) RegisterDevice(ctx context.Context, req sdk.RegisterDeviceRequest) (*sdk.Device, error) {
	return a.client.RegisterDevice(ctx, req)
}

func (a *GatewayAdapter) GetShadow(ctx context.Context, deviceID uint) (*sdk.Shadow, error) {
	return a.client.GetShadow(ctx, deviceID)
}

func (a *GatewayAdapter) IssueCommand(ctx context.Context, deviceID uint, req sdk.IssueCommandRequest) (*sdk.Command, error) {
	return a.client.IssueCommand(ctx, deviceID, req)
}

func (a *GatewayAdapter) ListDeviceCommands(ctx context.Context, deviceID uint, status string, page, pageSize int) ([]*sdk.Command, *sdk.Page, error) {
	return a.client.ListDeviceCommands(ctx, deviceID, status, page, pageSize)
}

func (a *GatewayAdapter) GetCommand(ctx context.Context, id uint) (*sdk.Command, error) {
	return a.client.GetCommand(ctx, id)
}

func (a *GatewayAdapter) QueryTelemetry(ctx context.Context, deviceID uint, q sdk.TelemetryQuery) (*sdk.TelemetryHistory, error) {
	return a.client.QueryTelemetry(ctx, deviceID, q)
}

func (a *GatewayAdapter) ListDataEvents(ctx context.Context, deviceID, sinceID uint, limit int) ([]*sdk.DataEvent, error) {
	return a.client.ListDataEvents(ctx, deviceID, sinceID, limit)
}
