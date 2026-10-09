// 数据映射网关适配器（平台开放面商户 HMAC，经 SDK）：SDK 类型 ↔ biz 镜像类型转换。
// 收敛口径（读=通用+本商户、写仅本商户独立 404、创建 merchant_id 平台注入）由平台保证；
// 404/409 等平台信封错误原样透传（同 DeleteModel 语义，不幂等放行），由 platformErr 映射。
package devmodel

import (
	"context"

	bizdevmodel "github.com/smilex/smilex-admin-gin/internal/biz/devmodel"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// MappingGatewayAdapter 开放面 SDK → biz 数据映射网关（类型镜像转换）
type MappingGatewayAdapter struct {
	client *platformsdk.Client
}

// NewMappingGatewayAdapter 构造（wire provider，绑定 bizdevmodel.MappingGateway）
func NewMappingGatewayAdapter(client *platformsdk.Client) *MappingGatewayAdapter {
	return &MappingGatewayAdapter{client: client}
}

func mappingDefFromSDK(d *platformsdk.DataMappingDef) *bizdevmodel.MappingDef {
	if d == nil {
		return nil
	}
	return &bizdevmodel.MappingDef{
		ID: d.ID, Name: d.Name, MerchantID: d.MerchantID,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func mappingFromSDK(m platformsdk.Mapping) bizdevmodel.Mapping {
	return bizdevmodel.Mapping{
		SourceTopic: m.SourceTopic, TopicMatch: m.TopicMatch,
		DataCategory: m.DataCategory, DataType: m.DataType,
		SampleRateHz: m.SampleRateHz, FieldExtract: m.FieldExtract,
		Fields: m.Fields, Trigger: m.Trigger,
	}
}

func mappingToSDK(m bizdevmodel.Mapping) platformsdk.Mapping {
	return platformsdk.Mapping{
		SourceTopic: m.SourceTopic, TopicMatch: m.TopicMatch,
		DataCategory: m.DataCategory, DataType: m.DataType,
		SampleRateHz: m.SampleRateHz, FieldExtract: m.FieldExtract,
		Fields: m.Fields, Trigger: m.Trigger,
	}
}

func mappingsFromSDK(ms []platformsdk.Mapping) []bizdevmodel.Mapping {
	if ms == nil {
		return nil
	}
	out := make([]bizdevmodel.Mapping, 0, len(ms))
	for _, m := range ms {
		out = append(out, mappingFromSDK(m))
	}
	return out
}

func mappingsToSDK(ms []bizdevmodel.Mapping) []platformsdk.Mapping {
	if ms == nil {
		return nil
	}
	out := make([]platformsdk.Mapping, 0, len(ms))
	for _, m := range ms {
		out = append(out, mappingToSDK(m))
	}
	return out
}

func mappingVersionFromSDK(v *platformsdk.DataMappingVersion) *bizdevmodel.MappingVersion {
	if v == nil {
		return nil
	}
	return &bizdevmodel.MappingVersion{
		ID: v.ID, DefID: v.DefID, Version: v.Version, Label: v.Label,
		Mappings: mappingsFromSDK(v.Mappings), Status: v.Status, RevertOf: v.RevertOf,
		PublishedAt: v.PublishedAt, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func (a *MappingGatewayAdapter) ListMappings(ctx context.Context, kw string, page, pageSize int) ([]*bizdevmodel.MappingDef, *pagination.Page, error) {
	list, pg, err := a.client.ListMappingDefs(ctx, kw, page, pageSize)
	if err != nil {
		return nil, nil, err
	}
	out := make([]*bizdevmodel.MappingDef, 0, len(list))
	for _, d := range list {
		out = append(out, mappingDefFromSDK(d))
	}
	return out, pageFromSDK(pg), nil
}

func (a *MappingGatewayAdapter) GetMapping(ctx context.Context, id uint) (*bizdevmodel.MappingDef, error) {
	d, err := a.client.GetMappingDef(ctx, id)
	if err != nil {
		return nil, err
	}
	return mappingDefFromSDK(d), nil
}

func (a *MappingGatewayAdapter) CreateMapping(ctx context.Context, req bizdevmodel.MappingCreate) (*bizdevmodel.MappingCreateResult, error) {
	res, err := a.client.CreateMappingDef(ctx, platformsdk.MappingCreateRequest{
		Name: req.Name, Label: req.Label, Mappings: mappingsToSDK(req.Mappings),
	})
	if err != nil {
		return nil, err
	}
	return &bizdevmodel.MappingCreateResult{
		Def:     mappingDefFromSDK(res.Def),
		Version: mappingVersionFromSDK(res.Version),
	}, nil
}

func (a *MappingGatewayAdapter) DeleteMapping(ctx context.Context, id uint) error {
	// 404 同时承载「不存在」与「通用/他商户资源不可写」，不幂等放行（同 DeleteModel）
	return a.client.DeleteMappingDef(ctx, id)
}

func (a *MappingGatewayAdapter) ListMappingVersions(ctx context.Context, defID uint) ([]*bizdevmodel.MappingVersion, error) {
	vs, err := a.client.ListMappingVersions(ctx, defID)
	if err != nil {
		return nil, err
	}
	out := make([]*bizdevmodel.MappingVersion, 0, len(vs))
	for _, v := range vs {
		out = append(out, mappingVersionFromSDK(v))
	}
	return out, nil
}

func (a *MappingGatewayAdapter) CreateMappingDraft(ctx context.Context, defID uint, req bizdevmodel.MappingDraft) (*bizdevmodel.MappingVersion, error) {
	v, err := a.client.CreateMappingDraft(ctx, defID, platformsdk.MappingDraftRequest{
		Label: req.Label, Mappings: mappingsToSDK(req.Mappings),
	})
	if err != nil {
		return nil, err
	}
	return mappingVersionFromSDK(v), nil
}

func (a *MappingGatewayAdapter) GetMappingVersion(ctx context.Context, versionID uint) (*bizdevmodel.MappingVersion, error) {
	v, err := a.client.GetMappingVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return mappingVersionFromSDK(v), nil
}

func (a *MappingGatewayAdapter) UpdateMappingDraft(ctx context.Context, versionID uint, req bizdevmodel.MappingDraftUpdate) error {
	return a.client.UpdateMappingDraft(ctx, versionID, platformsdk.MappingDraftUpdateRequest{
		Label: req.Label, Mappings: mappingsToSDK(req.Mappings),
	})
}

func (a *MappingGatewayAdapter) PublishMapping(ctx context.Context, versionID uint) (*bizdevmodel.MappingVersion, error) {
	v, err := a.client.PublishMapping(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return mappingVersionFromSDK(v), nil
}

func (a *MappingGatewayAdapter) RollbackMapping(ctx context.Context, versionID uint, publish bool) (*bizdevmodel.MappingVersion, error) {
	v, err := a.client.RollbackMapping(ctx, versionID, publish)
	if err != nil {
		return nil, err
	}
	return mappingVersionFromSDK(v), nil
}

func (a *MappingGatewayAdapter) DeleteMappingVersion(ctx context.Context, versionID uint) error {
	return a.client.DeleteMappingVersion(ctx, versionID)
}

func (a *MappingGatewayAdapter) ListMappingBoundModels(ctx context.Context, defID uint) ([]*bizdevmodel.Model, error) {
	list, err := a.client.ListMappingBoundModels(ctx, defID)
	if err != nil {
		return nil, err
	}
	out := make([]*bizdevmodel.Model, 0, len(list))
	for _, m := range list {
		out = append(out, modelFromSDK(m))
	}
	return out, nil
}

func (a *MappingGatewayAdapter) BindMappingModel(ctx context.Context, defID, modelID uint) error {
	return a.client.BindMappingModel(ctx, defID, modelID)
}

func (a *MappingGatewayAdapter) UnbindMappingModel(ctx context.Context, defID, modelID uint) error {
	return a.client.UnbindMappingModel(ctx, defID, modelID)
}

func (a *MappingGatewayAdapter) GetPublishedMapping(ctx context.Context, modelID uint) (*bizdevmodel.MappingVersion, error) {
	v, err := a.client.GetPublishedMapping(ctx, modelID)
	if err != nil {
		return nil, err
	}
	return mappingVersionFromSDK(v), nil // 无已发布版本：SDK 返回 nil，经 nil 安全的转换保持 nil, nil
}
