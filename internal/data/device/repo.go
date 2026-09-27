// Package device 设备网关适配器（平台开放面 SDK）
package device

import (
	"context"

	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
)

// GatewayAdapter 开放面 SDK → biz 设备网关
type GatewayAdapter struct {
	client *platformsdk.Client
}

// NewGatewayAdapter 构造（wire provider，绑定 bizdevice.Gateway）
func NewGatewayAdapter(client *platformsdk.Client) *GatewayAdapter {
	return &GatewayAdapter{client: client}
}

func (a *GatewayAdapter) ListDevices(ctx context.Context, page, pageSize int, filter platformsdk.DeviceFilter) ([]*platformsdk.Device, *platformsdk.Page, error) {
	return a.client.ListDevices(ctx, page, pageSize, filter)
}

func (a *GatewayAdapter) GetDevice(ctx context.Context, id uint) (*platformsdk.Device, error) {
	return a.client.GetDevice(ctx, id)
}

func (a *GatewayAdapter) RegisterDevice(ctx context.Context, req platformsdk.RegisterDeviceRequest) (*platformsdk.Device, error) {
	return a.client.RegisterDevice(ctx, req)
}

func (a *GatewayAdapter) GetShadow(ctx context.Context, deviceID uint) (*platformsdk.Shadow, error) {
	return a.client.GetShadow(ctx, deviceID)
}

func (a *GatewayAdapter) IssueCommand(ctx context.Context, deviceID uint, req platformsdk.IssueCommandRequest) (*platformsdk.Command, error) {
	return a.client.IssueCommand(ctx, deviceID, req)
}

func (a *GatewayAdapter) ListDeviceCommands(ctx context.Context, deviceID uint, status string, page, pageSize int) ([]*platformsdk.Command, *platformsdk.Page, error) {
	return a.client.ListDeviceCommands(ctx, deviceID, status, page, pageSize)
}

func (a *GatewayAdapter) GetCommand(ctx context.Context, id uint) (*platformsdk.Command, error) {
	return a.client.GetCommand(ctx, id)
}

func (a *GatewayAdapter) QueryTelemetry(ctx context.Context, deviceID uint, q platformsdk.TelemetryQuery) (*platformsdk.TelemetryHistory, error) {
	return a.client.QueryTelemetry(ctx, deviceID, q)
}

func (a *GatewayAdapter) ListDataEvents(ctx context.Context, deviceID, sinceID uint, limit int) ([]*platformsdk.DataEvent, error) {
	return a.client.ListDataEvents(ctx, deviceID, sinceID, limit)
}
