// Package devmodel 型号网关适配器（平台开放面商户 HMAC，经 SDK）。
// 2026-09-16 起平台开放面提供型号域（scope model:*）与物模型只读选择器
// （scope thing-model:read，版本仅 published），不再依赖管理面服务账号。
package devmodel

import (
	"context"

	bizdevmodel "github.com/smilex/smilex-admin-gin/internal/biz/devmodel"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

// GatewayAdapter 开放面 SDK → biz 型号网关（类型镜像转换）
type GatewayAdapter struct {
	client *sdk.Client
}

// NewGatewayAdapter 构造（wire provider，绑定 bizdevmodel.Gateway）
func NewGatewayAdapter(client *sdk.Client) *GatewayAdapter {
	return &GatewayAdapter{client: client}
}

func modelFromSDK(m *sdk.DeviceModel) *bizdevmodel.Model {
	if m == nil {
		return nil
	}
	return &bizdevmodel.Model{
		ID: m.ID, Code: m.Code, Name: m.Name,
		TMNodeID: m.TMNodeID, TMVersionID: m.TMVersionID,
		TMNodeName: m.TMNodeName, TMVersionNo: m.TMVersionNo,
		Status: m.Status, Manufacturer: m.Manufacturer,
		Description: m.Description, Transport: m.Transport,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func pageFromSDK(p *sdk.Page) *pagination.Page {
	if p == nil {
		return nil
	}
	return &pagination.Page{Page: p.Page, PageSize: p.PageSize, Total: p.Total}
}

func (a *GatewayAdapter) ListModels(ctx context.Context, q bizdevmodel.ModelQuery, page, pageSize int) ([]*bizdevmodel.Model, *pagination.Page, error) {
	list, pg, err := a.client.ListDeviceModels(ctx, page, pageSize, sdk.ModelFilter{
		Keyword: q.Keyword, Status: q.Status,
	})
	if err != nil {
		return nil, nil, err
	}
	out := make([]*bizdevmodel.Model, 0, len(list))
	for _, m := range list {
		out = append(out, modelFromSDK(m))
	}
	return out, pageFromSDK(pg), nil
}

func (a *GatewayAdapter) GetModel(ctx context.Context, id uint) (*bizdevmodel.Model, error) {
	m, err := a.client.GetDeviceModel(ctx, id)
	if err != nil {
		return nil, err
	}
	return modelFromSDK(m), nil
}

func (a *GatewayAdapter) CreateModel(ctx context.Context, req bizdevmodel.ModelCreate) (*bizdevmodel.Model, error) {
	m, err := a.client.CreateDeviceModel(ctx, sdk.ModelCreateRequest{
		Code: req.Code, Name: req.Name, TMNodeID: req.TMNodeID, TMVersionID: req.TMVersionID,
		Manufacturer: req.Manufacturer, Description: req.Description, Transport: req.Transport,
	})
	if err != nil {
		return nil, err
	}
	return modelFromSDK(m), nil
}

func (a *GatewayAdapter) UpdateModel(ctx context.Context, id uint, req bizdevmodel.ModelUpdate) error {
	return a.client.UpdateDeviceModel(ctx, id, sdk.ModelUpdateRequest{
		Name: req.Name, TMVersionID: req.TMVersionID, Status: req.Status,
		Manufacturer: req.Manufacturer, Description: req.Description, Transport: req.Transport,
	})
}

func (a *GatewayAdapter) DeleteModel(ctx context.Context, id uint) error {
	return a.client.DeleteDeviceModel(ctx, id)
}

func (a *GatewayAdapter) ListTMNodes(ctx context.Context, layer, kw string) ([]*bizdevmodel.TMNode, error) {
	// pageSize=0 全量（选择器整表构建）
	nodes, _, err := a.client.ListThingModelNodes(ctx, layer, kw, 0)
	if err != nil {
		return nil, err
	}
	out := make([]*bizdevmodel.TMNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, &bizdevmodel.TMNode{
			ID: n.ID, Layer: n.Layer, ParentID: n.ParentID,
			Code: n.Code, Name: n.Name, Status: n.Status,
		})
	}
	return out, nil
}

func (a *GatewayAdapter) ListTMVersions(ctx context.Context, nodeID uint) ([]*bizdevmodel.TMVersion, error) {
	// 开放面只返回 published（biz 用例另有 published 过滤，双保险幂等）
	vs, err := a.client.ListTMPublishedVersions(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	out := make([]*bizdevmodel.TMVersion, 0, len(vs))
	for _, v := range vs {
		out = append(out, &bizdevmodel.TMVersion{
			ID: v.ID, NodeID: v.NodeID, Version: v.Version,
			Status: v.Status, PublishedAt: v.PublishedAt,
		})
	}
	return out, nil
}
