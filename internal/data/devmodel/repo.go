// Package devmodel 型号网关适配器（平台管理面服务账号）
package devmodel

import (
	"context"

	bizdevmodel "github.com/smilex/smilex-admin-gin/internal/biz/devmodel"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// GatewayAdapter 管理面 AdminClient → biz 型号网关（类型镜像转换）
type GatewayAdapter struct {
	admin *platform.AdminClient
}

// NewGatewayAdapter 构造（wire provider，绑定 bizdevmodel.Gateway）
func NewGatewayAdapter(admin *platform.AdminClient) *GatewayAdapter {
	return &GatewayAdapter{admin: admin}
}

func modelFromPlatform(p *platform.DeviceModel) *bizdevmodel.Model {
	if p == nil {
		return nil
	}
	return &bizdevmodel.Model{
		ID: p.ID, Code: p.Code, Name: p.Name,
		TMNodeID: p.TMNodeID, TMVersionID: p.TMVersionID,
		TMNodeName: p.TMNodeName, TMVersionNo: p.TMVersionNo,
		Status: p.Status, Manufacturer: p.Manufacturer,
		Description: p.Description, Transport: p.Transport,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func pageFromPlatform(p *platform.Page) *pagination.Page {
	if p == nil {
		return nil
	}
	return &pagination.Page{Page: p.Page, PageSize: p.PageSize, Total: p.Total}
}

func (a *GatewayAdapter) ListModels(ctx context.Context, q bizdevmodel.ModelQuery, page, pageSize int) ([]*bizdevmodel.Model, *pagination.Page, error) {
	list, pg, err := a.admin.ListDeviceModels(ctx, platform.ModelQuery{Keyword: q.Keyword, Status: q.Status}, page, pageSize)
	if err != nil {
		return nil, nil, err
	}
	out := make([]*bizdevmodel.Model, 0, len(list))
	for _, m := range list {
		out = append(out, modelFromPlatform(m))
	}
	return out, pageFromPlatform(pg), nil
}

func (a *GatewayAdapter) GetModel(ctx context.Context, id uint) (*bizdevmodel.Model, error) {
	m, err := a.admin.GetDeviceModel(ctx, id)
	if err != nil {
		return nil, err
	}
	return modelFromPlatform(m), nil
}

func (a *GatewayAdapter) CreateModel(ctx context.Context, req bizdevmodel.ModelCreate) (*bizdevmodel.Model, error) {
	m, err := a.admin.CreateDeviceModel(ctx, platform.ModelCreate{
		Code: req.Code, Name: req.Name, TMNodeID: req.TMNodeID, TMVersionID: req.TMVersionID,
		Manufacturer: req.Manufacturer, Description: req.Description, Transport: req.Transport,
	})
	if err != nil {
		return nil, err
	}
	return modelFromPlatform(m), nil
}

func (a *GatewayAdapter) UpdateModel(ctx context.Context, id uint, req bizdevmodel.ModelUpdate) error {
	return a.admin.UpdateDeviceModel(ctx, id, platform.ModelUpdate{
		Name: req.Name, TMVersionID: req.TMVersionID, Status: req.Status,
		Manufacturer: req.Manufacturer, Description: req.Description, Transport: req.Transport,
	})
}

func (a *GatewayAdapter) DeleteModel(ctx context.Context, id uint) error {
	return a.admin.DeleteDeviceModel(ctx, id)
}

func (a *GatewayAdapter) ListTMNodes(ctx context.Context, layer, kw string) ([]*bizdevmodel.TMNode, error) {
	nodes, err := a.admin.ListThingModelNodes(ctx, layer, kw)
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
	vs, err := a.admin.ListThingModelVersions(ctx, nodeID)
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
