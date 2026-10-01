// Package skill 技能仓储 GORM 实现（主表 + 附属文件子表整体读写）
package skill

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/smilex/smilex-admin-gin/internal/biz/skill"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
)

type Repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) skill.Repo { return &Repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return skill.ErrNotFound
	}
	return err
}

// loadFiles 批量回填附属文件（按 skill_id 分组）
func (r *Repo) loadFiles(ctx context.Context, list []*skill.Skill) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(list))
	for _, s := range list {
		ids = append(ids, s.ID)
	}
	var pos []model.SkillFilePO
	if err := r.data.DB.WithContext(ctx).Where("skill_id IN ?", ids).Order("id ASC").Find(&pos).Error; err != nil {
		return err
	}
	files := make(map[uint][]model.SkillFilePO, len(list))
	for _, p := range pos {
		files[p.SkillID] = append(files[p.SkillID], p)
	}
	for _, s := range list {
		if fl, ok := files[s.ID]; ok {
			model.AttachSkillFiles(s, fl)
		}
	}
	return nil
}

func (r *Repo) Create(ctx context.Context, s *skill.Skill) error {
	po := model.SkillToPO(s)
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&po).Error; err != nil {
			return err
		}
		s.ID, s.CreatedAt, s.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
		if rows := model.SkillFilePOs(po.ID, s.Files); len(rows) > 0 {
			return tx.Create(&rows).Error
		}
		return nil
	})
}

func (r *Repo) Update(ctx context.Context, s *skill.Skill) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SkillPO{}).Where("id = ?", s.ID).Updates(map[string]interface{}{
			"name": s.Name, "code": s.Code, "description": s.Description,
			"instruction": s.Instruction, "remark": s.Remark, "status": s.Status,
		}).Error; err != nil {
			return err
		}
		// 附属文件整体替换（物理删后插，复合唯一索引不含软删行）
		if err := tx.Where("skill_id = ?", s.ID).Delete(&model.SkillFilePO{}).Error; err != nil {
			return err
		}
		if rows := model.SkillFilePOs(s.ID, s.Files); len(rows) > 0 {
			return tx.Create(&rows).Error
		}
		return nil
	})
}

func (r *Repo) Delete(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.SkillPO{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return skill.ErrNotFound
		}
		if err := tx.Where("skill_id = ?", id).Delete(&model.SkillFilePO{}).Error; err != nil {
			return err
		}
		// 软删行仍占用 name/code 唯一索引，归档释放以便重建
		return data.ArchiveUniqueColumns(tx, "skills", id, map[string]int{"name": 20, "code": 64})
	})
}

func (r *Repo) Find(ctx context.Context, id uint) (*skill.Skill, error) {
	var po model.SkillPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	s := model.SkillFromPO(&po)
	if err := r.loadFiles(ctx, []*skill.Skill{s}); err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repo) FindByCode(ctx context.Context, code string) (*skill.Skill, error) {
	var po model.SkillPO
	if err := r.data.DB.WithContext(ctx).Where("code = ?", code).First(&po).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.SkillFromPO(&po), nil // 唯一预检场景，无需装载文件
}

func (r *Repo) FindByName(ctx context.Context, name string) (*skill.Skill, error) {
	var po model.SkillPO
	if err := r.data.DB.WithContext(ctx).Where("name = ?", name).First(&po).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.SkillFromPO(&po), nil
}

func (r *Repo) List(ctx context.Context, q skill.Query, page, pageSize int) ([]*skill.Skill, int64, error) {
	db := r.data.DB.WithContext(ctx).Model(&model.SkillPO{})
	if kw := q.Kw; kw != "" {
		escaped := security.EscapeLike(kw)
		db = db.Where("name LIKE ? ESCAPE '/' OR code LIKE ? ESCAPE '/'",
			"%"+escaped+"%", "%"+escaped+"%")
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
	var pos []model.SkillPO
	if err := db.Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*skill.Skill, 0, len(pos))
	for i := range pos {
		out = append(out, model.SkillFromPO(&pos[i]))
	}
	if err := r.loadFiles(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *Repo) ListEnabled(ctx context.Context) ([]*skill.Skill, error) {
	var pos []model.SkillPO
	if err := r.data.DB.WithContext(ctx).Where("status = ?", skill.StatusEnabled).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*skill.Skill, 0, len(pos))
	for i := range pos {
		out = append(out, model.SkillFromPO(&pos[i]))
	}
	return out, nil
}

func (r *Repo) FindEnabledByCodes(ctx context.Context, codes []string) ([]*skill.Skill, error) {
	var pos []model.SkillPO
	if err := r.data.DB.WithContext(ctx).Where("code IN ? AND status = ?", codes, skill.StatusEnabled).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*skill.Skill, 0, len(pos))
	ids := make([]uint, 0, len(pos))
	for i := range pos {
		out = append(out, model.SkillFromPO(&pos[i]))
		ids = append(ids, pos[i].ID)
	}
	// 注入需要附属文件内容，批量装载
	if len(ids) > 0 {
		var files []model.SkillFilePO
		if err := r.data.DB.WithContext(ctx).Where("skill_id IN ?", ids).Order("id ASC").Find(&files).Error; err != nil {
			return nil, err
		}
		bySkill := make(map[uint][]model.SkillFilePO, len(out))
		for _, f := range files {
			bySkill[f.SkillID] = append(bySkill[f.SkillID], f)
		}
		for i, s := range out {
			if fl, ok := bySkill[s.ID]; ok {
				model.AttachSkillFiles(out[i], fl)
			}
		}
	}
	return out, nil
}

func (r *Repo) CountAgentsUsing(ctx context.Context, code string) (int64, error) {
	// agents.skills 为 JSON 字符串数组，带引号精确匹配（"code" 作为 JSON 字符串出现）
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.AgentPO{}).
		Where("skills LIKE ?", `%"`+code+`"%`).Count(&n).Error
	return n, err
}
