package skill

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// codeRe 编码约束：字母数字开头，可含下划线/中划线（与 MCP 服务编码同规）
var codeRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// pathRe 附属文件路径：相对路径段（字母数字开头，可含 . _ - 与子目录），禁绝对路径与 ..
var pathRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-]*(/[a-zA-Z0-9][a-zA-Z0-9._\-]*)*$`)

// 附属文件防护参数
const (
	maxFiles       = 10       // 单技能附属文件数上限
	maxFileContent = 32 << 10 // 单文件内容上限（字节）
)

// Usecase 技能领域用例：多文件技能包 CRUD + 智能体绑定支撑
type Usecase struct {
	repo Repo
}

func NewUsecase(repo Repo) *Usecase { return &Usecase{repo: repo} }

// SkillInput 写入参数（全量提交，含附属文件整体替换）
type SkillInput struct {
	Name        string
	Code        string
	Description string
	Instruction string
	Files       []SkillFile
	Remark      string
	Status      int
}

// normalizeFiles 清洗附属文件：校验路径合法、去空白行内容、查重、超量超长拦截
func normalizeFiles(files []SkillFile) ([]SkillFile, error) {
	out := make([]SkillFile, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		path := strings.TrimSpace(f.Path)
		if path == "" {
			continue // 空行直接丢弃（前端动态行可能留空）
		}
		if !pathRe.MatchString(path) || strings.Contains(path, "..") {
			return nil, ErrInvalidFilePath
		}
		if len(f.Content) > maxFileContent {
			return nil, ErrInvalidFilePath
		}
		if _, dup := seen[path]; dup {
			return nil, ErrDuplicatePath
		}
		if len(out) >= maxFiles {
			return nil, ErrInvalidFilePath
		}
		seen[path] = struct{}{}
		out = append(out, SkillFile{Path: path, Content: f.Content})
	}
	return out, nil
}

func (uc *Usecase) validate(in *SkillInput) error {
	if !codeRe.MatchString(in.Code) {
		return ErrInvalidCode
	}
	return nil
}

func (uc *Usecase) Create(ctx context.Context, in SkillInput) (*Skill, error) {
	if err := uc.validate(&in); err != nil {
		return nil, err
	}
	files, err := normalizeFiles(in.Files)
	if err != nil {
		return nil, err
	}
	if _, err := uc.repo.FindByCode(ctx, in.Code); err == nil {
		return nil, ErrCodeExists
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if _, err := uc.repo.FindByName(ctx, in.Name); err == nil {
		return nil, ErrNameExists
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	s := &Skill{
		Name: in.Name, Code: in.Code, Description: in.Description,
		Instruction: in.Instruction, Files: files, Remark: in.Remark, Status: in.Status,
	}
	if err := uc.repo.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (uc *Usecase) Update(ctx context.Context, id uint, in SkillInput) error {
	s, err := uc.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	if err := uc.validate(&in); err != nil {
		return err
	}
	files, err := normalizeFiles(in.Files)
	if err != nil {
		return err
	}
	if in.Code != s.Code {
		// 改码会使既有绑定失效：被引用时禁改
		if n, err := uc.repo.CountAgentsUsing(ctx, s.Code); err != nil {
			return err
		} else if n > 0 {
			return ErrSkillInUse
		}
		if _, err := uc.repo.FindByCode(ctx, in.Code); err == nil {
			return ErrCodeExists
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		s.Code = in.Code
	}
	if in.Name != s.Name {
		if _, err := uc.repo.FindByName(ctx, in.Name); err == nil {
			return ErrNameExists
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	s.Name, s.Description = in.Name, in.Description
	s.Instruction, s.Files = in.Instruction, files
	s.Remark, s.Status = in.Remark, in.Status
	return uc.repo.Update(ctx, s)
}

func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	s, err := uc.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	if n, err := uc.repo.CountAgentsUsing(ctx, s.Code); err != nil {
		return err
	} else if n > 0 {
		return ErrSkillInUse
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Skill, error) {
	return uc.repo.Find(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Skill, pagination.Page, error) {
	list, total, err := uc.repo.List(ctx, q, page, pageSize)
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// ---- 智能体接入（bizagent.SkillSource 实现） ----

// CheckEnabled 校验编码存在且启用（Agent 绑定引用时的强校验）
func (uc *Usecase) CheckEnabled(ctx context.Context, code string) error {
	s, err := uc.repo.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	if s.Status != StatusEnabled {
		return ErrSkillDisabled
	}
	return nil
}

// Contents 按绑定顺序返回启用技能的注入内容（缺失/禁用跳过）
func (uc *Usecase) Contents(ctx context.Context, codes []string) ([]SkillContent, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	list, err := uc.repo.FindEnabledByCodes(ctx, codes)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]*Skill, len(list))
	for _, s := range list {
		byCode[s.Code] = s
	}
	out := make([]SkillContent, 0, len(codes))
	for _, code := range codes {
		if s, ok := byCode[code]; ok {
			out = append(out, SkillContent{
				Name: s.Name, Description: s.Description, Instruction: s.Instruction, Files: s.Files,
			})
		}
	}
	return out, nil
}
