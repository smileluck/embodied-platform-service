package agent

// 智能体绑定技能：Agent.Skills 存技能 code 列表，聊天时经 SkillSource 拉取内容
// 拼接进 system prompt（与 MCP 工具形成「提示词能力 + 工具能力」互补）。

import (
	"context"
	"errors"

	bizskill "github.com/smilex/smilex-admin-gin/internal/biz/skill"
)

// 哨兵错误
var (
	ErrUnknownSkill  = errors.New("绑定的技能不存在或已禁用")
	ErrTooManySkills = errors.New("绑定的技能数量超过上限（10 个）")
)

// maxAgentSkills 单个 Agent 可绑定的技能数上限（注入总量另有 systemPromptMax 兜底）
const maxAgentSkills = 10

// SkillSource 技能源（跨上下文最小接口，实现见 skill 限界上下文的 Usecase）
type SkillSource interface {
	// CheckEnabled 校验技能 code 存在且启用（Agent 绑定引用时强校验）
	CheckEnabled(ctx context.Context, code string) error
	// Contents 按绑定顺序返回启用技能的注入内容（缺失/禁用跳过）
	Contents(ctx context.Context, codes []string) ([]bizskill.SkillContent, error)
}
