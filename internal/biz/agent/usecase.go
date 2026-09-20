package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Usecase 智能体领域用例：三层配置 CRUD + LLM 调用编排（harness 底座入口）
type Usecase struct {
	repo   Repo
	crypto *Crypto
}

func NewUsecase(repo Repo, cfg *conf.Bootstrap) *Usecase {
	return &Usecase{repo: repo, crypto: NewCrypto(cfg.Agent.CryptoKey, cfg.JWT.Secret)}
}

// ---- 供应商 ----

// ProviderInput 供应商写入参数（APIKey 明文仅在写入链路出现；更新时留空=保持不变）
type ProviderInput struct {
	Name    string
	Code    string
	BaseURL string
	APIKey  string
	Remark  string
	Status  Status
}

func (uc *Usecase) CreateProvider(ctx context.Context, in ProviderInput) (*Provider, error) {
	if _, err := uc.repo.FindProviderByCode(ctx, in.Code); err == nil {
		return nil, ErrProviderCodeExists
	} else if !errors.Is(err, ErrProviderNotFound) {
		return nil, err
	}
	enc, err := uc.crypto.Encrypt(in.APIKey)
	if err != nil {
		return nil, err
	}
	p := &Provider{
		Name: in.Name, Code: in.Code, BaseURL: strings.TrimRight(in.BaseURL, "/"),
		APIKeyEnc: enc, APIKeyMask: MaskKey(in.APIKey),
		Protocol: ProtocolOpenAI, Remark: in.Remark, Status: in.Status,
	}
	if err := uc.repo.CreateProvider(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *Usecase) UpdateProvider(ctx context.Context, id uint, in ProviderInput) error {
	p, err := uc.repo.FindProviderByID(ctx, id)
	if err != nil {
		return err
	}
	if in.Code != p.Code {
		if _, err := uc.repo.FindProviderByCode(ctx, in.Code); err == nil {
			return ErrProviderCodeExists
		} else if !errors.Is(err, ErrProviderNotFound) {
			return err
		}
		p.Code = in.Code
	}
	if in.Name != "" {
		p.Name = in.Name
	}
	if in.BaseURL != "" {
		p.BaseURL = strings.TrimRight(in.BaseURL, "/")
	}
	if in.Remark != "" {
		p.Remark = in.Remark
	}
	if in.APIKey != "" {
		enc, err := uc.crypto.Encrypt(in.APIKey)
		if err != nil {
			return err
		}
		p.APIKeyEnc, p.APIKeyMask = enc, MaskKey(in.APIKey)
	}
	p.Status = in.Status
	return uc.repo.UpdateProvider(ctx, p)
}

func (uc *Usecase) DeleteProvider(ctx context.Context, id uint) error {
	if n, err := uc.repo.CountModelsByProvider(ctx, id); err != nil {
		return err
	} else if n > 0 {
		return ErrProviderHasModels
	}
	return uc.repo.DeleteProvider(ctx, id)
}

func (uc *Usecase) GetProvider(ctx context.Context, id uint) (*Provider, error) {
	return uc.repo.FindProviderByID(ctx, id)
}

func (uc *Usecase) ListProviders(ctx context.Context, q ProviderQuery, page, pageSize int) ([]*Provider, pagination.Page, error) {
	list, total, err := uc.repo.ListProviders(ctx, q, page, pageSize)
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// ---- 模型 ----

// ModelInput 模型写入参数
type ModelInput struct {
	ProviderID    uint
	Name          string
	DisplayName   string
	ContextWindow int
	MaxOutput     int
	SupportsTools bool
	Remark        string
	Status        Status
}

func (uc *Usecase) CreateModel(ctx context.Context, in ModelInput) (*Model, error) {
	if _, err := uc.repo.FindProviderByID(ctx, in.ProviderID); err != nil {
		return nil, err
	}
	if _, err := uc.repo.FindModelByName(ctx, in.ProviderID, in.Name); err == nil {
		return nil, ErrModelExists
	} else if !errors.Is(err, ErrModelNotFound) {
		return nil, err
	}
	m := &Model{
		ProviderID: in.ProviderID, Name: in.Name, DisplayName: in.DisplayName,
		ContextWindow: in.ContextWindow, MaxOutput: in.MaxOutput, SupportsTools: in.SupportsTools,
		Remark: in.Remark, Status: in.Status,
	}
	if err := uc.repo.CreateModel(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (uc *Usecase) UpdateModel(ctx context.Context, id uint, in ModelInput) error {
	m, err := uc.repo.FindModelByID(ctx, id)
	if err != nil {
		return err
	}
	// 空=保持原值
	if in.ProviderID == 0 {
		in.ProviderID = m.ProviderID
	}
	if in.Name == "" {
		in.Name = m.Name
	}
	if _, err := uc.repo.FindProviderByID(ctx, in.ProviderID); err != nil {
		return err
	}
	if in.Name != m.Name || in.ProviderID != m.ProviderID {
		if _, err := uc.repo.FindModelByName(ctx, in.ProviderID, in.Name); err == nil {
			return ErrModelExists
		} else if !errors.Is(err, ErrModelNotFound) {
			return err
		}
	}
	m.ProviderID = in.ProviderID
	m.Name = in.Name
	if in.DisplayName != "" {
		m.DisplayName = in.DisplayName
	}
	m.ContextWindow = in.ContextWindow
	m.MaxOutput = in.MaxOutput
	m.SupportsTools = in.SupportsTools
	if in.Remark != "" {
		m.Remark = in.Remark
	}
	m.Status = in.Status
	return uc.repo.UpdateModel(ctx, m)
}

func (uc *Usecase) DeleteModel(ctx context.Context, id uint) error {
	if n, err := uc.repo.CountAgentsByModel(ctx, id); err != nil {
		return err
	} else if n > 0 {
		return ErrModelInUse
	}
	return uc.repo.DeleteModel(ctx, id)
}

func (uc *Usecase) GetModel(ctx context.Context, id uint) (*Model, error) {
	return uc.repo.FindModelByID(ctx, id)
}

func (uc *Usecase) ListModels(ctx context.Context, q ModelQuery, page, pageSize int) ([]*Model, pagination.Page, error) {
	list, total, err := uc.repo.ListModels(ctx, q, page, pageSize)
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// ---- Agent ----

// AgentInput Agent 写入参数
type AgentInput struct {
	Name         string
	Code         string
	ModelID      uint
	SystemPrompt string
	Temperature  float64
	TopP         float64
	MaxTokens    int
	Remark       string
	Status       Status
}

func (uc *Usecase) CreateAgent(ctx context.Context, in AgentInput) (*Agent, error) {
	if _, err := uc.repo.FindModelByID(ctx, in.ModelID); err != nil {
		return nil, err
	}
	if _, err := uc.repo.FindAgentByCode(ctx, in.Code); err == nil {
		return nil, ErrAgentCodeExists
	} else if !errors.Is(err, ErrAgentNotFound) {
		return nil, err
	}
	a := &Agent{
		Name: in.Name, Code: in.Code, ModelID: in.ModelID,
		SystemPrompt: in.SystemPrompt, Temperature: in.Temperature, TopP: in.TopP,
		MaxTokens: in.MaxTokens, Remark: in.Remark, Status: in.Status,
	}
	if err := uc.repo.CreateAgent(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (uc *Usecase) UpdateAgent(ctx context.Context, id uint, in AgentInput) error {
	a, err := uc.repo.FindAgentByID(ctx, id)
	if err != nil {
		return err
	}
	// 空=保持原值
	if in.ModelID == 0 {
		in.ModelID = a.ModelID
	}
	if _, err := uc.repo.FindModelByID(ctx, in.ModelID); err != nil {
		return err
	}
	if in.Code != a.Code {
		if _, err := uc.repo.FindAgentByCode(ctx, in.Code); err == nil {
			return ErrAgentCodeExists
		} else if !errors.Is(err, ErrAgentNotFound) {
			return err
		}
		a.Code = in.Code
	}
	if in.Name != "" {
		a.Name = in.Name
	}
	a.ModelID = in.ModelID
	a.SystemPrompt = in.SystemPrompt
	a.Temperature = in.Temperature
	a.TopP = in.TopP
	a.MaxTokens = in.MaxTokens
	if in.Remark != "" {
		a.Remark = in.Remark
	}
	a.Status = in.Status
	return uc.repo.UpdateAgent(ctx, a)
}

func (uc *Usecase) DeleteAgent(ctx context.Context, id uint) error {
	return uc.repo.DeleteAgent(ctx, id)
}

func (uc *Usecase) GetAgent(ctx context.Context, id uint) (*Agent, error) {
	return uc.repo.FindAgentByID(ctx, id)
}

func (uc *Usecase) ListAgents(ctx context.Context, q AgentQuery, page, pageSize int) ([]*Agent, pagination.Page, error) {
	list, total, err := uc.repo.ListAgents(ctx, q, page, pageSize)
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// ---- 测试与对话（LLM 运行时编排） ----

// TestResult 连通性测试结果
type TestResult struct {
	Content   string `json:"content"`
	Model     string `json:"model"`
	LatencyMs int64  `json:"latency_ms"`
	Usage     Usage  `json:"usage"`
}

// testPrompt 连通性测试固定提示词（限制输出长度，控制 token 消耗）
const testPrompt = "这是一条连通性测试消息，请直接回复：连接成功"

// TestProvider 供应商连通性测试：modelID 为 0 时取该供应商首个启用模型。
// 不校验启用状态：禁用中的配置也允许先验证再启用。
func (uc *Usecase) TestProvider(ctx context.Context, providerID, modelID uint) (*TestResult, error) {
	p, err := uc.repo.FindProviderByID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	m, err := uc.resolveModel(ctx, p.ID, modelID)
	if err != nil {
		return nil, err
	}
	cli, err := uc.clientFor(p)
	if err != nil {
		return nil, err
	}
	return uc.test(ctx, cli, m)
}

// TestModel 模型连通性测试（按模型定位供应商）
func (uc *Usecase) TestModel(ctx context.Context, modelID uint) (*TestResult, error) {
	m, err := uc.repo.FindModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	p, err := uc.repo.FindProviderByID(ctx, m.ProviderID)
	if err != nil {
		return nil, err
	}
	cli, err := uc.clientFor(p)
	if err != nil {
		return nil, err
	}
	return uc.test(ctx, cli, m)
}

// RemoteModels 拉取上游模型列表（录入辅助）
func (uc *Usecase) RemoteModels(ctx context.Context, providerID uint) ([]string, error) {
	p, err := uc.repo.FindProviderByID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	cli, err := uc.clientFor(p)
	if err != nil {
		return nil, err
	}
	return cli.ListModels(ctx)
}

// ChatMeta 对话元信息（SSE 首帧，前端展示模型归属）
type ChatMeta struct {
	AgentID    uint   `json:"agent_id"`
	AgentName  string `json:"agent_name"`
	ProviderID uint   `json:"provider_id"`
	ModelID    uint   `json:"model_id"`
	Model      string `json:"model"`
}

// ChatStream Agent 调试对话（Playground）：system prompt 由 Agent 配置注入，
// 历史消息由前端带入（role 仅 user/assistant，system 不可注入）；无状态，不落库。
func (uc *Usecase) ChatStream(ctx context.Context, agentID uint, history []Message) (<-chan StreamEvent, *ChatMeta, error) {
	a, err := uc.repo.FindAgentByID(ctx, agentID)
	if err != nil {
		return nil, nil, err
	}
	if a.Status != StatusEnabled {
		return nil, nil, ErrAgentDisabled
	}
	m, err := uc.repo.FindModelByID(ctx, a.ModelID)
	if err != nil {
		return nil, nil, err
	}
	if m.Status != StatusEnabled {
		return nil, nil, ErrModelDisabled
	}
	p, err := uc.repo.FindProviderByID(ctx, m.ProviderID)
	if err != nil {
		return nil, nil, err
	}
	if p.Status != StatusEnabled {
		return nil, nil, ErrProviderDisabled
	}
	cli, err := uc.clientFor(p)
	if err != nil {
		return nil, nil, err
	}

	msgs := make([]Message, 0, len(history)+1)
	if a.SystemPrompt != "" {
		msgs = append(msgs, Message{Role: "system", Content: a.SystemPrompt})
	}
	msgs = append(msgs, history...)

	ch, err := cli.StreamCompletion(ctx, ChatRequest{
		Model: m.Name, Messages: msgs,
		Temperature: a.Temperature, TopP: a.TopP, MaxTokens: a.MaxTokens,
	})
	if err != nil {
		return nil, nil, err
	}
	meta := &ChatMeta{AgentID: a.ID, AgentName: a.Name, ProviderID: p.ID, ModelID: m.ID, Model: m.Name}
	return ch, meta, nil
}

// resolveModel 供应商下定位测试模型：modelID 为 0 取首个启用模型，否则须归属该供应商
func (uc *Usecase) resolveModel(ctx context.Context, providerID, modelID uint) (*Model, error) {
	if modelID == 0 {
		return uc.repo.FindFirstEnabledModel(ctx, providerID)
	}
	m, err := uc.repo.FindModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if m.ProviderID != providerID {
		return nil, ErrModelNotFound
	}
	return m, nil
}

// clientFor 解密密钥并构建客户端（本地服务如 Ollama 可不配密钥）
func (uc *Usecase) clientFor(p *Provider) (LLMClient, error) {
	key, err := uc.crypto.Decrypt(p.APIKeyEnc)
	if err != nil {
		return nil, err
	}
	return NewLLMClient(p, key), nil
}

func (uc *Usecase) test(ctx context.Context, cli LLMClient, m *Model) (*TestResult, error) {
	start := time.Now()
	resp, err := cli.ChatCompletion(ctx, ChatRequest{
		Model: m.Name, Messages: []Message{{Role: "user", Content: testPrompt}}, MaxTokens: 64,
	})
	if err != nil {
		return nil, err
	}
	return &TestResult{
		Content: resp.Content, Model: resp.Model,
		LatencyMs: time.Since(start).Milliseconds(), Usage: resp.Usage,
	}, nil
}
