// Package agent 智能体仓储 GORM 实现
package agent

import (
	"context"
	"errors"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/biz/agent"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

type Repo struct {
	data               *data.Data
	usageRetentionDays int
}

// NewRepo 创建智能体仓储（用量流水保留期清理已迁移为定时任务 handler，见 biz/job）
func NewRepo(d *data.Data, c *conf.Bootstrap) *Repo {
	return &Repo{data: d, usageRetentionDays: c.Agent.UsageRetentionDays}
}

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

func (r *Repo) CreateProvider(ctx context.Context, p *agent.Provider) error {
	po := model.AgentProviderToPO(p)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	p.ID, p.CreatedAt, p.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *Repo) UpdateProvider(ctx context.Context, p *agent.Provider) error {
	return r.data.DB.WithContext(ctx).Model(&model.AgentProviderPO{}).Where("id = ?", p.ID).
		Updates(map[string]interface{}{
			"name": p.Name, "code": p.Code, "base_url": p.BaseURL,
			"api_key_enc": p.APIKeyEnc, "api_key_mask": p.APIKeyMask,
			"protocol": p.Protocol, "remark": p.Remark, "status": int(p.Status),
		}).Error
}

func (r *Repo) DeleteProvider(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.AgentProviderPO{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return agent.ErrProviderNotFound
		}
		// 软删行仍占用 code 唯一索引，归档释放以便同编码重建
		return data.ArchiveUniqueColumns(tx, "agent_providers", id, "code")
	})
}

func (r *Repo) FindProviderByID(ctx context.Context, id uint) (*agent.Provider, error) {
	var po model.AgentProviderPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapProviderErr(err)
	}
	return model.AgentProviderFromPO(&po), nil
}

func (r *Repo) FindProviderByCode(ctx context.Context, code string) (*agent.Provider, error) {
	var po model.AgentProviderPO
	if err := r.data.DB.WithContext(ctx).Where("code = ?", code).First(&po).Error; err != nil {
		return nil, mapProviderErr(err)
	}
	return model.AgentProviderFromPO(&po), nil
}

func (r *Repo) ListProviders(ctx context.Context, q agent.ProviderQuery, page, pageSize int) ([]*agent.Provider, int64, error) {
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
	if err := tx.Scopes(paginate(page, pageSize)).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*agent.Provider, 0, len(pos))
	for i := range pos {
		out = append(out, model.AgentProviderFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *Repo) CountModelsByProvider(ctx context.Context, providerID uint) (int64, error) {
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.AgentModelPO{}).
		Where("provider_id = ?", providerID).Count(&n).Error
	return n, err
}

// ---- 模型 ----

func (r *Repo) CreateModel(ctx context.Context, m *agent.Model) error {
	po := model.AgentModelToPO(m)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	m.ID, m.CreatedAt, m.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *Repo) UpdateModel(ctx context.Context, m *agent.Model) error {
	return r.data.DB.WithContext(ctx).Model(&model.AgentModelPO{}).Where("id = ?", m.ID).
		Updates(map[string]interface{}{
			"provider_id": m.ProviderID, "name": m.Name, "display_name": m.DisplayName,
			"context_window": m.ContextWindow, "max_output": m.MaxOutput,
			"supports_tools": m.SupportsTools, "input_price": m.InputPrice, "output_price": m.OutputPrice,
			"remark": m.Remark, "status": int(m.Status),
		}).Error
}

func (r *Repo) DeleteModel(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.AgentModelPO{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return agent.ErrModelNotFound
		}
		// 软删行仍占用 (provider_id,name) 复合唯一索引，归档释放以便重建
		return data.ArchiveUniqueColumns(tx, "agent_models", id, "name")
	})
}

func (r *Repo) FindModelByID(ctx context.Context, id uint) (*agent.Model, error) {
	var po model.AgentModelPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapModelErr(err)
	}
	return model.AgentModelFromPO(&po), nil
}

func (r *Repo) FindModelByName(ctx context.Context, providerID uint, name string) (*agent.Model, error) {
	var po model.AgentModelPO
	if err := r.data.DB.WithContext(ctx).
		Where("provider_id = ? AND name = ?", providerID, name).First(&po).Error; err != nil {
		return nil, mapModelErr(err)
	}
	return model.AgentModelFromPO(&po), nil
}

func (r *Repo) FindFirstEnabledModel(ctx context.Context, providerID uint) (*agent.Model, error) {
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

func (r *Repo) ListModels(ctx context.Context, q agent.ModelQuery, page, pageSize int) ([]*agent.Model, int64, error) {
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
	if err := tx.Scopes(paginate(page, pageSize)).Order("id ASC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*agent.Model, 0, len(pos))
	for i := range pos {
		out = append(out, model.AgentModelFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *Repo) CountAgentsByModel(ctx context.Context, modelID uint) (int64, error) {
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.AgentPO{}).
		Where("model_id = ?", modelID).Count(&n).Error
	return n, err
}

// ---- Agent ----

func (r *Repo) CreateAgent(ctx context.Context, a *agent.Agent) error {
	po := model.AgentToPO(a)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	a.ID, a.CreatedAt, a.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *Repo) UpdateAgent(ctx context.Context, a *agent.Agent) error {
	return r.data.DB.WithContext(ctx).Model(&model.AgentPO{}).Where("id = ?", a.ID).
		Updates(map[string]interface{}{
			"name": a.Name, "code": a.Code, "model_id": a.ModelID,
			"system_prompt": a.SystemPrompt, "temperature": a.Temperature, "top_p": a.TopP,
			"max_tokens": a.MaxTokens, "tools": model.MarshalAgentTools(a.Tools), "remark": a.Remark, "status": int(a.Status),
		}).Error
}

func (r *Repo) DeleteAgent(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.AgentPO{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return agent.ErrAgentNotFound
		}
		// 软删行仍占用 code 唯一索引，归档释放以便同编码重建
		return data.ArchiveUniqueColumns(tx, "agents", id, "code")
	})
}

func (r *Repo) FindAgentByID(ctx context.Context, id uint) (*agent.Agent, error) {
	var po model.AgentPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapAgentErr(err)
	}
	return model.AgentFromPO(&po), nil
}

func (r *Repo) FindAgentByCode(ctx context.Context, code string) (*agent.Agent, error) {
	var po model.AgentPO
	if err := r.data.DB.WithContext(ctx).Where("code = ?", code).First(&po).Error; err != nil {
		return nil, mapAgentErr(err)
	}
	return model.AgentFromPO(&po), nil
}

func (r *Repo) ListAgents(ctx context.Context, q agent.AgentQuery, page, pageSize int) ([]*agent.Agent, int64, error) {
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
	if err := tx.Scopes(paginate(page, pageSize)).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*agent.Agent, 0, len(pos))
	for i := range pos {
		out = append(out, model.AgentFromPO(&pos[i]))
	}
	return out, total, nil
}

// ---- 会话（user_id 过滤强制在仓储层；不存在与无权限统一返回 ErrConversationNotFound） ----

func mapConversationErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agent.ErrConversationNotFound
	}
	return err
}

func (r *Repo) CreateConversation(ctx context.Context, cv *agent.Conversation) error {
	po := model.AgentConversationToPO(cv)
	po.CreatedAt, po.UpdatedAt = po.LastMsgAt, po.LastMsgAt
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	cv.ID, cv.CreatedAt, cv.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *Repo) UpdateConversation(ctx context.Context, cv *agent.Conversation) error {
	return r.data.DB.WithContext(ctx).Model(&model.AgentConversationPO{}).Where("id = ?", cv.ID).
		Updates(map[string]interface{}{
			"title": cv.Title, "agent_name": cv.AgentName, "last_msg_at": cv.LastMsgAt,
		}).Error
}

// DeleteConversation 事务：软删会话 + 物理删除其全部消息（追加流水无需留痕）
func (r *Repo) DeleteConversation(ctx context.Context, userID, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.AgentConversationPO{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return agent.ErrConversationNotFound
		}
		return tx.Where("conversation_id = ?", id).Delete(&model.AgentConversationMsgPO{}).Error
	})
}

func (r *Repo) FindConversation(ctx context.Context, userID, id uint) (*agent.Conversation, error) {
	var po model.AgentConversationPO
	if err := r.data.DB.WithContext(ctx).Where("user_id = ?", userID).First(&po, id).Error; err != nil {
		return nil, mapConversationErr(err)
	}
	return model.AgentConversationFromPO(&po), nil
}

func (r *Repo) ListConversations(ctx context.Context, userID uint, q agent.ConversationQuery, page, pageSize int) ([]*agent.Conversation, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.AgentConversationPO{}).Where("user_id = ?", userID)
	if q.AgentID != nil {
		tx = tx.Where("agent_id = ?", *q.AgentID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.AgentConversationPO
	if err := tx.Scopes(paginate(page, pageSize)).
		Order("last_msg_at DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*agent.Conversation, 0, len(pos))
	for i := range pos {
		out = append(out, model.AgentConversationFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *Repo) AppendMessage(ctx context.Context, m *agent.ConversationMessage) error {
	po := model.AgentMsgToPO(m)
	po.CreatedAt = time.Now()
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	m.ID, m.CreatedAt = po.ID, po.CreatedAt
	return nil
}

func (r *Repo) ListMessages(ctx context.Context, conversationID uint, page, pageSize int) ([]*agent.ConversationMessage, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.AgentConversationMsgPO{}).Where("conversation_id = ?", conversationID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.AgentConversationMsgPO
	if err := tx.Scopes(paginate(page, pageSize)).
		Order("id ASC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*agent.ConversationMessage, 0, len(pos))
	for i := range pos {
		out = append(out, model.AgentMsgFromPO(&pos[i]))
	}
	return out, total, nil
}

// ---- 用量计量 ----

func (r *Repo) AppendUsage(ctx context.Context, u *agent.UsageLog) error {
	po := model.AgentUsageToPO(u)
	po.CreatedAt = time.Now()
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	u.ID, u.CreatedAt = po.ID, po.CreatedAt
	return nil
}

func (r *Repo) ListUsageSince(ctx context.Context, since time.Time) ([]*agent.UsageLog, error) {
	var pos []model.AgentUsageLogPO
	if err := r.data.DB.WithContext(ctx).Where("created_at >= ?", since).
		Order("id ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*agent.UsageLog, 0, len(pos))
	for i := range pos {
		out = append(out, model.AgentUsageFromPO(&pos[i]))
	}
	return out, nil
}

func (r *Repo) CleanupUsageBefore(ctx context.Context, before time.Time) error {
	return r.data.DB.WithContext(ctx).Where("created_at < ?", before).
		Delete(&model.AgentUsageLogPO{}).Error
}

// CleanupExpired 清理保留期外用量流水（定时任务 handler 调用；保留期<=0 永久保留）
func (r *Repo) CleanupExpired(ctx context.Context) error {
	if r.usageRetentionDays <= 0 {
		return nil
	}
	return r.CleanupUsageBefore(ctx, time.Now().AddDate(0, 0, -r.usageRetentionDays))
}

// paginate pageSize<=0 表示全量（引用数据整表拉取）；否则按页取
func paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if pageSize > 0 {
			return db.Offset((page - 1) * pageSize).Limit(pageSize)
		}
		return db
	}
}
