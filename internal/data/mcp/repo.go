// Package mcp MCP 服务器配置仓储 GORM 实现
package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"github.com/smilex/smilex-admin-gin/internal/biz/mcp"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
)

type Repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) mcp.Repo { return &Repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return mcp.ErrNotFound
	}
	return err
}

func (r *Repo) Create(ctx context.Context, s *mcp.Server) error {
	po := model.McpServerToPO(s)
	if err := r.data.DB.WithContext(ctx).Create(&po).Error; err != nil {
		return err
	}
	s.ID, s.CreatedAt, s.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *Repo) Update(ctx context.Context, s *mcp.Server) error {
	headers, _ := json.Marshal(s.Headers)
	return r.data.DB.WithContext(ctx).Model(&model.McpServerPO{}).Where("id = ?", s.ID).
		Updates(map[string]interface{}{
			"name": s.Name, "code": s.Code, "transport": string(s.Transport), "base_url": s.BaseURL,
			"headers": string(headers), "token_enc": s.TokenEnc, "token_mask": s.TokenMask,
			"remark": s.Remark, "status": s.Status,
		}).Error
}

func (r *Repo) Delete(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.McpServerPO{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return mcp.ErrNotFound
		}
		// 软删行仍占用 name/code 唯一索引，归档释放以便重建
		return data.ArchiveUniqueColumns(tx, "mcp_servers", id, map[string]int{"name": 20, "code": 64})
	})
}

func (r *Repo) Find(ctx context.Context, id uint) (*mcp.Server, error) {
	var po model.McpServerPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.McpServerFromPO(&po), nil
}

func (r *Repo) FindByCode(ctx context.Context, code string) (*mcp.Server, error) {
	var po model.McpServerPO
	if err := r.data.DB.WithContext(ctx).Where("code = ?", code).First(&po).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.McpServerFromPO(&po), nil
}

func (r *Repo) FindByName(ctx context.Context, name string) (*mcp.Server, error) {
	var po model.McpServerPO
	if err := r.data.DB.WithContext(ctx).Where("name = ?", name).First(&po).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.McpServerFromPO(&po), nil
}

func (r *Repo) List(ctx context.Context, q mcp.Query, page, pageSize int) ([]*mcp.Server, int64, error) {
	db := r.data.DB.WithContext(ctx).Model(&model.McpServerPO{})
	if kw := q.Kw; kw != "" {
		escaped := security.EscapeLike(kw)
		db = db.Where("name LIKE ? ESCAPE '/' OR code LIKE ? ESCAPE '/'",
			"%"+escaped+"%", "%"+escaped+"%")
	}
	if q.Transport != "" {
		db = db.Where("transport = ?", q.Transport)
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page > 0 && pageSize > 0 {
		db = db.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	var pos []model.McpServerPO
	if err := db.Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*mcp.Server, 0, len(pos))
	for i := range pos {
		out = append(out, model.McpServerFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *Repo) ListEnabled(ctx context.Context) ([]*mcp.Server, error) {
	var pos []model.McpServerPO
	if err := r.data.DB.WithContext(ctx).Where("status = ?", mcp.StatusEnabled).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*mcp.Server, 0, len(pos))
	for i := range pos {
		out = append(out, model.McpServerFromPO(&pos[i]))
	}
	return out, nil
}

func (r *Repo) CountAgentsUsing(ctx context.Context, code string) (int64, error) {
	// agents.tools 为 JSON 字符串数组，按命名空间前缀模糊匹配（前缀足够特异；宁可误报拦删）
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.AgentPO{}).
		Where("tools LIKE ?", "%mcp:"+code+":%").Count(&n).Error
	return n, err
}
