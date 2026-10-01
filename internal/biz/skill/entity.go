// Package skill 技能限界上下文 —— 领域层。
// 结构化提示词模块（多文件技能包）：主指令（instruction）+ 附属文件（reference/scripts 等文本资源），
// 智能体绑定后聊天时拼接到 system prompt，与 MCP 工具形成「提示词能力 + 工具能力」互补。
package skill

import (
	"context"
	"errors"
	"time"
)

// 哨兵错误
var (
	ErrNotFound        = errors.New("技能不存在")
	ErrCodeExists      = errors.New("技能编码已存在")
	ErrNameExists      = errors.New("技能名称已存在")
	ErrInvalidCode     = errors.New("技能编码仅限字母数字开头，可含下划线与中划线")
	ErrInvalidFilePath = errors.New("附属文件路径不合法（相对路径，禁止 .. 与绝对路径）")
	ErrDuplicatePath   = errors.New("附属文件路径重复")
	ErrSkillInUse      = errors.New("技能已被智能体绑定，请先解除引用")
	ErrSkillDisabled   = errors.New("技能已禁用")
)

// 状态值（与全局约定一致：1 启用 0 禁用）
const (
	StatusDisabled = 0
	StatusEnabled  = 1
)

// SkillFile 附属文件（文本资源，随技能整体提交替换）
type SkillFile struct {
	Path    string `json:"path"`    // 相对路径（如 reference.md / scripts/helper.py）
	Content string `json:"content"` // 文本内容
}

// Size 内容字节数（展示用）
func (f SkillFile) Size() int { return len(f.Content) }

// Skill 技能（json tag 与前端类型字段对齐；列表/详情含附属文件元信息）
type Skill struct {
	ID          uint        `json:"id"`
	Name        string      `json:"name"`
	Code        string      `json:"code"` // 稳定引用（Agent 以 code 绑定）
	Description string      `json:"description"`
	Instruction string      `json:"instruction"` // 主指令（SKILL.md 等价物，Markdown）
	Files       []SkillFile `json:"files"`
	Remark      string      `json:"remark"`
	Status      int         `json:"status"` // 1 启用 0 禁用
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Query 管理端列表条件
type Query struct {
	Kw     string // 名称/编码模糊
	Status *int
}

// SkillContent 注入用技能内容（agent 上下文消费；禁用/缺失的技能不出现在结果中）
type SkillContent struct {
	Name        string
	Description string
	Instruction string
	Files       []SkillFile
}

// Repo 仓储接口
type Repo interface {
	Create(ctx context.Context, s *Skill) error
	Update(ctx context.Context, s *Skill) error
	// Delete 软删主表（归档唯一列）并物理删附属文件
	Delete(ctx context.Context, id uint) error
	Find(ctx context.Context, id uint) (*Skill, error)
	FindByCode(ctx context.Context, code string) (*Skill, error)
	FindByName(ctx context.Context, name string) (*Skill, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*Skill, int64, error)
	// ListEnabled 全部启用技能（Agent 表单绑定选项）
	ListEnabled(ctx context.Context) ([]*Skill, error)
	// FindEnabledByCodes 按绑定顺序返回启用的技能（缺失/禁用跳过）
	FindEnabledByCodes(ctx context.Context, codes []string) ([]*Skill, error)
	// CountAgentsUsing Agent 绑定预检：skills 数组包含 code 的智能体数（JSON 带引号精确匹配）
	CountAgentsUsing(ctx context.Context, code string) (int64, error)
}
