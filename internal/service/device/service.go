// Package device 设备管理应用服务（平台开放面代理）
package device

import (
	"context"
	"encoding/json"
	"time"

	bizdevice "github.com/smilex/smilex-admin-gin/internal/biz/device"
	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

type Service struct {
	uc *bizdevice.Usecase
}

func NewService(uc *bizdevice.Usecase) *Service { return &Service{uc: uc} }

// ListRequest 列表查询（租户过滤不可指定：由平台按商户绑定租户集服务端收敛）
type ListRequest struct {
	Keyword   string `form:"kw"`
	ModelID   uint   `form:"model_id"`
	Status    string `form:"status"`
	Online    *bool  `form:"online"`
	Transport string `form:"transport"`
}

func (s *Service) List(ctx context.Context, req ListRequest, page, pageSize int) ([]*sdk.Device, interface{}, error) {
	return s.uc.List(ctx, page, pageSize, sdk.DeviceFilter{
		Keyword: req.Keyword, ModelID: req.ModelID, Status: req.Status,
		Online: req.Online, Transport: req.Transport,
	})
}

// RegisterRequest 注册设备入参（TenantID 为本地租户同步而得的平台租户 ID）
type RegisterRequest struct {
	SN              string            `json:"sn" binding:"required,max=128"`
	Name            string            `json:"name" binding:"required,max=64"`
	ModelID         uint              `json:"model_id"`
	TenantID        uint              `json:"tenant_id" binding:"required"`
	FirmwareVersion string            `json:"firmware_version" binding:"max=64"`
	HardwareVersion string            `json:"hardware_version" binding:"max=64"`
	ServiceVersions map[string]string `json:"service_versions"`
	Transport       string            `json:"transport" binding:"omitempty,oneof=socket mqtt"`
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*sdk.Device, error) {
	return s.uc.Register(ctx, sdk.RegisterDeviceRequest{
		SN: req.SN, Name: req.Name, ModelID: req.ModelID, TenantID: req.TenantID,
		FirmwareVersion: req.FirmwareVersion, HardwareVersion: req.HardwareVersion,
		ServiceVersions: req.ServiceVersions, Transport: req.Transport,
	})
}

func (s *Service) Get(ctx context.Context, id uint) (*sdk.Device, error) { return s.uc.Get(ctx, id) }

func (s *Service) Shadow(ctx context.Context, id uint) (*sdk.Shadow, error) {
	return s.uc.Shadow(ctx, id)
}

// IssueCommandRequest 命令下发入参（priority 必填：0 急停 / 1 运维 / 2 Agent / 3 业务）
type IssueCommandRequest struct {
	CommandType    string          `json:"command_type" binding:"required"`
	Params         json.RawMessage `json:"params"`
	Priority       *int            `json:"priority" binding:"required,min=0,max=3"`
	IdempotencyKey string          `json:"idempotency_key" binding:"max=64"`
}

func (s *Service) IssueCommand(ctx context.Context, deviceID uint, req IssueCommandRequest) (*sdk.Command, error) {
	priority := 3
	if req.Priority != nil {
		priority = *req.Priority
	}
	return s.uc.IssueCommand(ctx, deviceID, sdk.IssueCommandRequest{
		CommandType: req.CommandType, Params: req.Params,
		Priority: priority, IdempotencyKey: req.IdempotencyKey,
	})
}

func (s *Service) ListCommands(ctx context.Context, deviceID uint, status string, page, pageSize int) ([]*sdk.Command, interface{}, error) {
	return s.uc.ListCommands(ctx, deviceID, status, page, pageSize)
}

func (s *Service) GetCommand(ctx context.Context, id uint) (*sdk.Command, error) {
	return s.uc.GetCommand(ctx, id)
}

// TelemetryRequest 遥测历史查询（from/to 为 RFC3339）
type TelemetryRequest struct {
	Metric          string `form:"metric"`
	From            string `form:"from"`
	To              string `form:"to"`
	IntervalSeconds int    `form:"interval_seconds"`
	Marker          string `form:"marker"`
	Limit           int    `form:"limit"`
}

func (s *Service) Telemetry(ctx context.Context, deviceID uint, req TelemetryRequest) (*sdk.TelemetryHistory, error) {
	q := sdk.TelemetryQuery{
		Metric: req.Metric, IntervalSeconds: req.IntervalSeconds,
		Marker: req.Marker, Limit: req.Limit,
	}
	if t, err := time.Parse(time.RFC3339, req.From); err == nil {
		q.From = t
	}
	if t, err := time.Parse(time.RFC3339, req.To); err == nil {
		q.To = t
	}
	return s.uc.Telemetry(ctx, deviceID, q)
}

func (s *Service) DataEvents(ctx context.Context, deviceID, sinceID uint, limit int) ([]*sdk.DataEvent, error) {
	return s.uc.DataEvents(ctx, deviceID, sinceID, limit)
}
