package agent

// 技能链接入单测：绑定校验（存在且启用/上限）、system prompt 注入格式（主指令 + 附属文件）、
// 禁用跳过、超预算截断。

import (
	"context"
	"strings"
	"testing"

	bizskill "github.com/smilex/smilex-admin-gin/internal/biz/skill"
	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// fakeSkillSource 内存技能源
type fakeSkillSource struct {
	skills map[string]*bizskill.SkillContent
}

func (f *fakeSkillSource) CheckEnabled(_ context.Context, code string) error {
	if _, ok := f.skills[code]; ok {
		return nil
	}
	return bizskill.ErrNotFound
}

func (f *fakeSkillSource) Contents(_ context.Context, codes []string) ([]bizskill.SkillContent, error) {
	out := make([]bizskill.SkillContent, 0, len(codes))
	for _, c := range codes {
		if s, ok := f.skills[c]; ok {
			out = append(out, *s)
		}
	}
	return out, nil
}

func newUsecaseWithSkills(repo Repo, src SkillSource) *Usecase {
	return NewUsecase(repo, &conf.Bootstrap{
		Agent: conf.Agent{CryptoKey: "unit-test-key"},
		JWT:   conf.JWT{Secret: "jwt-secret"},
	}, nil, nil, src)
}

func newFakeSkills() *fakeSkillSource {
	return &fakeSkillSource{skills: map[string]*bizskill.SkillContent{
		"writer": {
			Name: "写作规范", Description: "统一文案口径",
			Instruction: "所有回复使用简体中文，语气专业友好。",
			Files:       []bizskill.SkillFile{{Path: "glossary.md", Content: "术语表：API=接口"}},
		},
		"pdf": {Name: "PDF 处理", Description: "", Instruction: "处理 PDF 文档时先解析目录。"},
	}}
}

func TestValidateSkills(t *testing.T) {
	uc := newUsecaseWithSkills(newFakeRepo(), newFakeSkills())
	ctx := context.Background()

	got, err := uc.validateSkills(ctx, []string{"writer", "pdf", "writer"})
	if err != nil || len(got) != 2 {
		t.Fatalf("validateSkills: %v %v", got, err)
	}
	if _, err := uc.validateSkills(ctx, []string{"ghost"}); err == nil {
		t.Fatal("unknown skill should fail")
	}
	many := make([]string, 11)
	for i := range many {
		many[i] = "writer"
	}
	if _, err := uc.validateSkills(ctx, many); err == nil {
		t.Fatal("11 skills should exceed limit")
	}
	if got, err := uc.validateSkills(ctx, nil); err != nil || got != nil {
		t.Fatalf("nil skills: %v %v", got, err)
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	uc := newUsecaseWithSkills(newFakeRepo(), newFakeSkills())
	a := &Agent{
		SystemPrompt: "你是一个助手。",
		Skills:       []string{"writer", "pdf"},
	}
	got := uc.buildSystemPrompt(context.Background(), a)

	for _, want := range []string{
		"你是一个助手。",
		"# 技能：写作规范",
		"统一文案口径",
		"所有回复使用简体中文，语气专业友好。",
		"## 附：glossary.md",
		"术语表：API=接口",
		"# 技能：PDF 处理",
		"处理 PDF 文档时先解析目录。",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("system prompt missing %q:\n%s", want, got)
		}
	}
	// 绑定顺序：写作规范块在 PDF 块之前
	if strings.Index(got, "# 技能：写作规范") > strings.Index(got, "# 技能：PDF 处理") {
		t.Fatal("skill blocks out of order")
	}
}

func TestBuildSystemPromptEmpty(t *testing.T) {
	uc := newUsecaseWithSkills(newFakeRepo(), newFakeSkills())
	// 无提示词无技能 -> 空串（不注入 system 消息）
	if got := uc.buildSystemPrompt(context.Background(), &Agent{}); got != "" {
		t.Fatalf("expect empty prompt, got %q", got)
	}
	// 无提示词有技能 -> 仅技能内容
	got := uc.buildSystemPrompt(context.Background(), &Agent{Skills: []string{"writer"}})
	if !strings.HasPrefix(got, "# 技能：写作规范") {
		t.Fatalf("unexpected prompt: %q", got)
	}
}

func TestBuildSystemPromptTruncated(t *testing.T) {
	src := &fakeSkillSource{skills: map[string]*bizskill.SkillContent{
		"big": {Name: "超大技能", Instruction: strings.Repeat("A", systemPromptMax+100)},
	}}
	uc := newUsecaseWithSkills(newFakeRepo(), src)
	got := uc.buildSystemPrompt(context.Background(), &Agent{Skills: []string{"big"}})
	if len(got) > systemPromptMax+100 { // 截断标记允许少量超出
		t.Fatalf("expect truncated prompt, got %d bytes", len(got))
	}
	if !strings.Contains(got, "已截断") {
		t.Fatal("truncation marker missing")
	}
}
