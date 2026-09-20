// Package agent 智能体仓储 GORM 实现
package agent

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/internal/biz/agent"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

type repo struct {
	data *data.Data
}

// NewRepo 创建智能体仓储
func NewRepo(d *data.Data) agent.Repo { return &repo{data: d} }

func mapProviderErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent.ErrProviderNotFound
	}
	return err
}

func mapModelErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent.ErrModelNotFound
	}
	return err
}

func mapAgentErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent.ErrAgentNotFound
	}
	return err
}

// ---- 供应商 ----

func (r *repo) CreateProvider(ctx context.Context, p *agent.Provider) error {
	po := model.AgentProviderToPO(p)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	p.ID, p.CreatedAt, p.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) UpdateProvider(ctx context.Context, p *agent.Provider) error {
	return r.data.DB.WithContext(ctx).Model(&model.AgentProviderPO{}).Where("id = ?", p.ID).
		Updates(map[string]interface{}{
			"name": p.Name, "code": p.Code, "base_url": p.BaseURL,
			"api_key_enc": p.APIKeyEnc, "api_key_mask": p.APIKeyMask,
			"protocol": p.Protocol, "remark": p.Remark, "status": int(p.Status),
		}).Error
}

func (r *repo) DeleteProvider(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.AgentProviderPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return agent.ErrProviderNotFound
	}
	return nil
}

func (r *repo) FindProviderByID(ctx context.Context, id uint) (*agent.Provider, error) {
	var po model.AgentProviderPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapProviderErr(err)
	}
	return model.AgentProviderFromPO(&po), nil
}

func (r *repo) FindProviderByCode(ctx context.Context, code string) (*agent.Provider, error) {
	var po model.AgentProviderPO
	if err := r.data.DB.WithContext(ctx).Where("code = ?", code).First(&po).Error; err != nil {
		return nil, mapProviderErr(err)
	}
	return model.AgentProviderFromPO(&po), nil
}

func (r *repo) ListProviders(ctx context.Context, q agent.ProviderQuery, page, pageSize int) ([]*agent.Provider, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.AgentProviderPO{})
	if q.Name != "" {
		tx = tx.Where("name LIKE ? ESCAPE '/'", security.EscapeLike(q.Name)+"%")
	}
	if q.Code != "" {
		tx = tx.Where("code LIKE ? ESCAPE '/'", security.EscapeLike(q.Code)+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.AgentProviderPO
	if err := tx.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*agent.Provider, 0, len(pos))
	for i := range pos {
		out = append(out, model.AgentProviderFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *repo) CountModelsByProvider(ctx context.Context, providerID uint) (int64, error) {
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.AgentModelPO{}).
		Where("provider_id = ?", providerID).Count(&n).Error
	return n, err
}

// ---- 模型 ----

func (r *repo) CreateModel(ctx context.Context, m *agent.Model) error {
	po := model.AgentModelToPO(m)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	m.ID, m.CreatedAt, m.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) UpdateModel(ctx context.Context, m *agent.Model) error {
	return r.data.DB.WithContext(ctx).Model(&model.AgentModelPO{}).Where("id = ?", m.ID).
		Updates(map[string]interface{}{
			"provider_id": m.ProviderID, "name": m.Name, "display_name": m.DisplayName,
			"context_window": m.ContextWindow, "max_output": m.MaxOutput,
			"supports_tools": m.SupportsTools, "remark": m.Remark, "status": int(m.Status),
		}).Error
}

func (r *repo) DeleteModel(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.AgentModelPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return agent.ErrModelNotFound
	}
	return nil
}

func (r *repo) FindModelByID(ctx context.Context, id uint) (*agent.Model, error) {
	var po model.AgentModelPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapModelErr(err)
	}
	return model.AgentModelFromPO(&po), nil
}

func (r *repo) FindModelByName(ctx context.Context, providerID uint, name string) (*agent.Model, error) {
	var po model.AgentModelPO
	if err := r.data.DB.WithContext(ctx).
		Where("provider_id = ? AND name = ?", providerID, name).First(&po).Error; err != nil {
		return nil, mapModelErr(err)
	}
	return model.AgentModelFromPO(&po), nil
}

func (r *repo) FindFirstEnabledModel(ctx context.Context, providerID uint) (*agent.Model, error) {
	var po model.AgentModelPO
	if err := r.data.DB.WithContext(ctx).
		Where("provider_id = ? AND status = ?", providerID, int(agent.StatusEnabled)).
		Order("id ASC").First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, agent.ErrProviderNoModels
		}
		return nil, err
	}
	return model.AgentModelFromPO(&po), nil
}

func (r *repo) ListModels(ctx context.Context, q agent.ModelQuery, page, pageSize int) ([]*agent.Model, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.AgentModelPO{})
	if q.ProviderID != nil {
		tx = tx.Where("provider_id = ?", *q.ProviderID)
	}
	if q.Name != "" {
		tx = tx.Where("name LIKE ? ESCAPE '/'", security.EscapeLike(q.Name)+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.AgentModelPO
	if err := tx.Offset((page - 1) * pageSize).Limit(pageSize).Order("id ASC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*agent.Model, 0, len(pos))
	for i := range pos {
		out = append(out, model.AgentModelFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *repo) CountAgentsByModel(ctx context.Context, modelID uint) (int64, error) {
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.AgentPO{}).
		Where("model_id = ?", modelID).Count(&n).Error
	return n, err
}

// ---- Agent ----

func (r *repo) CreateAgent(ctx context.Context, a *agent.Agent) error {
	po := model.AgentToPO(a)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	a.ID, a.CreatedAt, a.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) UpdateAgent(ctx context.Context, a *agent.Agent) error {
	return r.data.DB.WithContext(ctx).Model(&model.AgentPO{}).Where("id = ?", a.ID).
		Updates(map[string]interface{}{
			"name": a.Name, "code": a.Code, "model_id": a.ModelID,
			"system_prompt": a.SystemPrompt, "temperature": a.Temperature, "top_p": a.TopP,
			"max_tokens": a.MaxTokens, "remark": a.Remark, "status": int(a.Status),
		}).Error
}

func (r *repo) DeleteAgent(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.AgentPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return agent.ErrAgentNotFound
	}
	return nil
}

func (r *repo) FindAgentByID(ctx context.Context, id uint) (*agent.Agent, error) {
	var po model.AgentPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapAgentErr(err)
	}
	return model.AgentFromPO(&po), nil
}

func (r *repo) FindAgentByCode(ctx context.Context, code string) (*agent.Agent, error) {
	var po model.AgentPO
	if err := r.data.DB.WithContext(ctx).Where("code = ?", code).First(&po).Error; err != nil {
		return nil, mapAgentErr(err)
	}
	return model.AgentFromPO(&po), nil
}

func (r *repo) ListAgents(ctx context.Context, q agent.AgentQuery, page, pageSize int) ([]*agent.Agent, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.AgentPO{})
	if q.Name != "" {
		tx = tx.Where("name LIKE ? ESCAPE '/'", security.EscapeLike(q.Name)+"%")
	}
	if q.Code != "" {
		tx = tx.Where("code LIKE ? ESCAPE '/'", security.EscapeLike(q.Code)+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.AgentPO
	if err := tx.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*agent.Agent, 0, len(pos))
	for i := range pos {
		out = append(out, model.AgentFromPO(&pos[i]))
	}
	return out, total, nil
}
