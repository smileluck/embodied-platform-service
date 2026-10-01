// Package skill 技能应用服务（多文件技能包管理）
package skill

import (
	"context"
	"strings"

	bizskill "github.com/smilex/smilex-admin-gin/internal/biz/skill"
)

type Service struct {
	uc *bizskill.Usecase
}

func NewService(uc *bizskill.Usecase) *Service { return &Service{uc: uc} }

// FileRequest 附属文件写入参数
type FileRequest struct {
	Path    string `json:"path" binding:"required,max=128"`
	Content string `json:"content" binding:"max=32768"`
}

// CreateRequest 新增（instruction 主指令必填；files 可选）
type CreateRequest struct {
	Name        string        `json:"name" binding:"required,max=20"`
	Code        string        `json:"code" binding:"required,max=64"`
	Description string        `json:"description" binding:"max=200"`
	Instruction string        `json:"instruction" binding:"required,max=8000"`
	Files       []FileRequest `json:"files" binding:"omitempty,max=10,dive"`
	Remark      string        `json:"remark" binding:"max=200"`
	Status      *int          `json:"status" binding:"omitempty,gte=0,lte=1"` // 缺省启用
}

type UpdateRequest struct {
	Name        string        `json:"name" binding:"required,max=20"`
	Code        string        `json:"code" binding:"required,max=64"`
	Description string        `json:"description" binding:"max=200"`
	Instruction string        `json:"instruction" binding:"required,max=8000"`
	Files       []FileRequest `json:"files" binding:"omitempty,max=10,dive"`
	Remark      string        `json:"remark" binding:"max=200"`
	Status      *int          `json:"status" binding:"omitempty,gte=0,lte=1"`
}

func toFiles(in []FileRequest) []bizskill.SkillFile {
	out := make([]bizskill.SkillFile, 0, len(in))
	for _, f := range in {
		out = append(out, bizskill.SkillFile{Path: strings.TrimSpace(f.Path), Content: f.Content})
	}
	return out
}

func statusOf(p *int) int {
	if p == nil {
		return bizskill.StatusEnabled
	}
	return *p
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*bizskill.Skill, error) {
	return s.uc.Create(ctx, bizskill.SkillInput{
		Name: strings.TrimSpace(req.Name), Code: strings.TrimSpace(req.Code),
		Description: strings.TrimSpace(req.Description), Instruction: req.Instruction,
		Files: toFiles(req.Files), Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) Update(ctx context.Context, id uint, req UpdateRequest) error {
	return s.uc.Update(ctx, id, bizskill.SkillInput{
		Name: strings.TrimSpace(req.Name), Code: strings.TrimSpace(req.Code),
		Description: strings.TrimSpace(req.Description), Instruction: req.Instruction,
		Files: toFiles(req.Files), Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) Delete(ctx context.Context, id uint) error { return s.uc.Delete(ctx, id) }
func (s *Service) Get(ctx context.Context, id uint) (*bizskill.Skill, error) {
	return s.uc.Get(ctx, id)
}
func (s *Service) List(ctx context.Context, q bizskill.Query, page, pageSize int) ([]*bizskill.Skill, interface{}, error) {
	return s.uc.List(ctx, q, page, pageSize)
}
