// 物模型网关适配器扩展（读面扩展 + 写面；同 GatewayAdapter，SDK 类型 ↔ biz 镜像类型转换）。
// 收敛口径（读=通用+本商户、写仅本商户独立 404、创建 merchant_id 平台注入）由平台保证；
// 404/409 等平台信封错误原样透传（不幂等放行），由 platformErr 映射。
package devmodel

import (
	"context"

	bizdevmodel "github.com/smilex/smilex-admin-gin/internal/biz/devmodel"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
)

func tmNodeFromSDK(n *platformsdk.TMNode) *bizdevmodel.TMNode {
	if n == nil {
		return nil
	}
	return &bizdevmodel.TMNode{
		ID: n.ID, Layer: n.Layer, ParentID: n.ParentID,
		Code: n.Code, Name: n.Name, Status: n.Status,
		MerchantID: n.MerchantID,
	}
}

func tmSchemaFromSDK(s *platformsdk.TMSchema) *bizdevmodel.TMSchema {
	if s == nil {
		return nil
	}
	out := &bizdevmodel.TMSchema{}
	if len(s.Properties) > 0 {
		out.Properties = make(map[string]bizdevmodel.TMProperty, len(s.Properties))
		for k, p := range s.Properties {
			out.Properties[k] = bizdevmodel.TMProperty{
				DataType: p.DataType, Unit: p.Unit, Writable: p.Writable,
				Enum: p.Enum, Min: p.Min, Max: p.Max, Description: p.Description,
			}
		}
	}
	if len(s.Services) > 0 {
		out.Services = make(map[string]bizdevmodel.TMService, len(s.Services))
		for k, v := range s.Services {
			out.Services[k] = bizdevmodel.TMService{
				CallType: v.CallType, Params: tmParamsFromSDK(v.Params), Description: v.Description,
			}
		}
	}
	if len(s.Events) > 0 {
		out.Events = make(map[string]bizdevmodel.TMEvent, len(s.Events))
		for k, v := range s.Events {
			out.Events[k] = bizdevmodel.TMEvent{
				Params: tmParamsFromSDK(v.Params), Description: v.Description,
			}
		}
	}
	return out
}

func tmParamsFromSDK(in map[string]platformsdk.TMParam) map[string]bizdevmodel.TMParam {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]bizdevmodel.TMParam, len(in))
	for k, p := range in {
		out[k] = bizdevmodel.TMParam{
			DataType: p.DataType, Required: p.Required, Enum: p.Enum, Description: p.Description,
		}
	}
	return out
}

func tmSchemaToSDK(s *bizdevmodel.TMSchema) *platformsdk.TMSchema {
	if s == nil {
		return nil
	}
	out := &platformsdk.TMSchema{}
	if len(s.Properties) > 0 {
		out.Properties = make(map[string]platformsdk.TMProperty, len(s.Properties))
		for k, p := range s.Properties {
			out.Properties[k] = platformsdk.TMProperty{
				DataType: p.DataType, Unit: p.Unit, Writable: p.Writable,
				Enum: p.Enum, Min: p.Min, Max: p.Max, Description: p.Description,
			}
		}
	}
	if len(s.Services) > 0 {
		out.Services = make(map[string]platformsdk.TMService, len(s.Services))
		for k, v := range s.Services {
			out.Services[k] = platformsdk.TMService{
				CallType: v.CallType, Params: tmParamsToSDK(v.Params), Description: v.Description,
			}
		}
	}
	if len(s.Events) > 0 {
		out.Events = make(map[string]platformsdk.TMEvent, len(s.Events))
		for k, v := range s.Events {
			out.Events[k] = platformsdk.TMEvent{
				Params: tmParamsToSDK(v.Params), Description: v.Description,
			}
		}
	}
	return out
}

func tmParamsToSDK(in map[string]bizdevmodel.TMParam) map[string]platformsdk.TMParam {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]platformsdk.TMParam, len(in))
	for k, p := range in {
		out[k] = platformsdk.TMParam{
			DataType: p.DataType, Required: p.Required, Enum: p.Enum, Description: p.Description,
		}
	}
	return out
}

func tmVersionFromSDK(v *platformsdk.TMVersion) *bizdevmodel.TMVersion {
	if v == nil {
		return nil
	}
	return &bizdevmodel.TMVersion{
		ID: v.ID, NodeID: v.NodeID, Version: v.Version,
		Schema: tmSchemaFromSDK(v.Schema), Status: v.Status,
		ParentVersions: v.ParentVersions, RevertOf: v.RevertOf,
		PublishedAt: v.PublishedAt, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func (a *GatewayAdapter) GetTMNode(ctx context.Context, nodeID uint) (*bizdevmodel.TMNode, error) {
	n, err := a.client.GetTMNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return tmNodeFromSDK(n), nil
}

func (a *GatewayAdapter) CreateTMNode(ctx context.Context, req bizdevmodel.TMNodeCreate) (*bizdevmodel.TMNode, error) {
	n, err := a.client.CreateTMNode(ctx, &platformsdk.TMNodeCreateRequest{
		Layer: req.Layer, ParentID: req.ParentID, Code: req.Code, Name: req.Name,
	})
	if err != nil {
		return nil, err
	}
	return tmNodeFromSDK(n), nil
}

func (a *GatewayAdapter) UpdateTMNode(ctx context.Context, nodeID uint, req bizdevmodel.TMNodeUpdate) error {
	return a.client.UpdateTMNode(ctx, nodeID, &platformsdk.TMNodeUpdateRequest{
		Name: req.Name, Status: req.Status,
	})
}

func (a *GatewayAdapter) DeleteTMNode(ctx context.Context, nodeID uint) error {
	// 404 同时承载「不存在」与「通用/他商户节点不可写」，不幂等放行（同 DeleteModel）
	return a.client.DeleteTMNode(ctx, nodeID)
}

func (a *GatewayAdapter) CreateTMDraft(ctx context.Context, nodeID uint) (*bizdevmodel.TMVersion, error) {
	v, err := a.client.CreateTMDraft(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return tmVersionFromSDK(v), nil
}

func (a *GatewayAdapter) UpdateTMDraft(ctx context.Context, versionID uint, schema *bizdevmodel.TMSchema) error {
	return a.client.UpdateTMDraft(ctx, versionID, tmSchemaToSDK(schema))
}

func (a *GatewayAdapter) PublishTMVersion(ctx context.Context, versionID uint) (*bizdevmodel.TMVersion, error) {
	v, err := a.client.PublishTMVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return tmVersionFromSDK(v), nil
}

func (a *GatewayAdapter) RollbackTMVersion(ctx context.Context, versionID uint, publish bool) (*bizdevmodel.TMVersion, error) {
	v, err := a.client.RollbackTMVersion(ctx, versionID, publish)
	if err != nil {
		return nil, err
	}
	return tmVersionFromSDK(v), nil
}

func (a *GatewayAdapter) DeleteTMVersion(ctx context.Context, versionID uint) error {
	return a.client.DeleteTMVersion(ctx, versionID)
}

func (a *GatewayAdapter) ResolveTM(ctx context.Context, nodeID, pinnedVersion uint) (*bizdevmodel.TMResolveResult, error) {
	res, err := a.client.ResolveTM(ctx, nodeID, pinnedVersion)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	out := &bizdevmodel.TMResolveResult{Schema: tmSchemaFromSDK(res.Schema)}
	for _, l := range res.Chain {
		out.Chain = append(out.Chain, &bizdevmodel.TMChainLink{
			Node: tmNodeFromSDK(l.Node), Version: tmVersionFromSDK(l.Version),
		})
	}
	return out, nil
}

func (a *GatewayAdapter) GetTMInheritanceStatus(ctx context.Context, nodeID uint) (*bizdevmodel.TMInheritanceStatus, error) {
	st, err := a.client.GetTMInheritanceStatus(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, nil
	}
	out := &bizdevmodel.TMInheritanceStatus{UpToDate: st.UpToDate, Baseline: st.Baseline}
	for _, u := range st.Updates {
		out.Updates = append(out.Updates, &bizdevmodel.TMInheritanceUpdate{
			Node:            tmNodeFromSDK(u.Node),
			SnapshotVersion: tmVersionFromSDK(u.SnapshotVersion),
			LatestVersion:   tmVersionFromSDK(u.LatestVersion),
		})
	}
	return out, nil
}
