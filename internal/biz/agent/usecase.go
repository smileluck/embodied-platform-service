package agent

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"go.uber.org/zap"
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
	// 每千 token 单价；nil=保持原值（更新）/未设置（新建），显式 0 表示清除
	InputPrice  *float64
	OutputPrice *float64
	Remark      string
	Status      Status
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
	if in.InputPrice != nil {
		m.InputPrice = *in.InputPrice
	}
	if in.OutputPrice != nil {
		m.OutputPrice = *in.OutputPrice
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
	if in.InputPrice != nil {
		m.InputPrice = *in.InputPrice
	}
	if in.OutputPrice != nil {
		m.OutputPrice = *in.OutputPrice
	}
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
	AgentID        uint   `json:"agent_id"`
	AgentName      string `json:"agent_name"`
	ProviderID     uint   `json:"provider_id"`
	ModelID        uint   `json:"model_id"`
	Model          string `json:"model"`
	ConversationID uint   `json:"conversation_id,omitempty"` // 持久化会话（0=无状态调用）
}

// ChatStream Agent 调试对话：system prompt 由 Agent 配置注入，
// 历史消息由前端带入（role 仅 user/assistant，system 不可注入）。
// conversationID > 0 时接入持久化：user 消息在流建立后立即落库，
// assistant 回复（含 usage）在流结束后落库并刷新会话活跃时间；中途停止时已生成部分照常保存。
func (uc *Usecase) ChatStream(ctx context.Context, agentID uint, userID, conversationID uint, history []Message) (<-chan StreamEvent, *ChatMeta, error) {
	start := time.Now()
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

	// 会话归属校验（本人 + Agent 匹配），在调用上游之前完成
	var conv *Conversation
	if conversationID > 0 {
		conv, err = uc.repo.FindConversation(ctx, userID, conversationID)
		if err != nil {
			return nil, nil, err
		}
		if conv.AgentID != agentID {
			return nil, nil, ErrConversationAgentMismatch
		}
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

	// 上游接受请求后再落 user 消息：避免上游拒绝时留下无回复的孤儿消息。
	// 末条历史即本轮新输入（调用方保证末条为 user）
	if conv != nil {
		uc.recordUserMessage(conv, history)
		ch = uc.persistAssistant(conv, ch)
	}
	// 用量计量对全部调用生效（含无状态调试）：流结束带 usage 时落一条流水
	ch = uc.recordUsage(agentID, m.ID, userID, start, ch)

	meta := &ChatMeta{AgentID: a.ID, AgentName: a.Name, ProviderID: p.ID, ModelID: m.ID, Model: m.Name}
	if conv != nil {
		meta.ConversationID = conv.ID
	}
	return ch, meta, nil
}

// recordUserMessage 落库本轮 user 消息；默认标题的会话用消息前 20 字命名
func (uc *Usecase) recordUserMessage(conv *Conversation, history []Message) {
	if len(history) == 0 || history[len(history)-1].Role != MsgRoleUser {
		return
	}
	last := history[len(history)-1]
	if err := uc.repo.AppendMessage(context.Background(), &ConversationMessage{
		ConversationID: conv.ID, Role: MsgRoleUser, Content: last.Content,
	}); err != nil {
		logger.Warn("append user message failed", zap.Uint("conversation", conv.ID), zap.Error(err))
	}
	if conv.Title == DefaultConversationTitle {
		if t := truncateRunes(strings.TrimSpace(last.Content), ConversationTitleMax); t != "" {
			conv.Title = t
		}
	}
	conv.LastMsgAt = time.Now()
	if err := uc.repo.UpdateConversation(context.Background(), conv); err != nil {
		logger.Warn("touch conversation failed", zap.Uint("conversation", conv.ID), zap.Error(err))
	}
}

// persistAssistant 包装流通道：透传事件的同时聚合 assistant 回复，
// 通道关闭后落库（出错/停止时已生成部分也保存）；DB 写失败仅告警不影响流输出
func (uc *Usecase) persistAssistant(conv *Conversation, in <-chan StreamEvent) <-chan StreamEvent {
	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		var content strings.Builder
		var usage int
		for ev := range in {
			content.WriteString(ev.Delta)
			if ev.Usage != nil {
				usage = ev.Usage.TotalTokens
			}
			out <- ev
		}
		if content.Len() == 0 {
			return
		}
		if err := uc.repo.AppendMessage(context.Background(), &ConversationMessage{
			ConversationID: conv.ID, Role: MsgRoleAssistant, Content: content.String(), TotalTokens: usage,
		}); err != nil {
			logger.Warn("append assistant message failed", zap.Uint("conversation", conv.ID), zap.Error(err))
		}
	}()
	return out
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

// ---- 会话（调试对话持久化；本人数据） ----

// ConversationTitleMax 会话标题长度上限（与输入长度约定一致：名称 ≤20）
const ConversationTitleMax = 20

// CreateConversation 新建会话：校验 Agent 存在且启用，标题为默认值（首条消息到达后自动改写）
func (uc *Usecase) CreateConversation(ctx context.Context, userID, agentID uint) (*Conversation, error) {
	a, err := uc.repo.FindAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if a.Status != StatusEnabled {
		return nil, ErrAgentDisabled
	}
	now := time.Now()
	cv := &Conversation{
		UserID: userID, AgentID: a.ID, AgentName: a.Name,
		Title: DefaultConversationTitle, LastMsgAt: now,
	}
	if err := uc.repo.CreateConversation(ctx, cv); err != nil {
		return nil, err
	}
	return cv, nil
}

// RenameConversation 会话改名（仅本人；长度截断到 20 字）
func (uc *Usecase) RenameConversation(ctx context.Context, userID, id uint, title string) (*Conversation, error) {
	cv, err := uc.repo.FindConversation(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	cv.Title = truncateRunes(strings.TrimSpace(title), ConversationTitleMax)
	if cv.Title == "" {
		cv.Title = DefaultConversationTitle
	}
	if err := uc.repo.UpdateConversation(ctx, cv); err != nil {
		return nil, err
	}
	return cv, nil
}

// DeleteConversation 删除会话（连带全部消息；仅本人）
func (uc *Usecase) DeleteConversation(ctx context.Context, userID, id uint) error {
	return uc.repo.DeleteConversation(ctx, userID, id)
}

// ListConversations 本人会话列表（LastMsgAt 倒序）
func (uc *Usecase) ListConversations(ctx context.Context, userID uint, q ConversationQuery, page, pageSize int) ([]*Conversation, pagination.Page, error) {
	list, total, err := uc.repo.ListConversations(ctx, userID, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// ListConversationMessages 会话消息（时间正序分页；仅本人）
func (uc *Usecase) ListConversationMessages(ctx context.Context, userID, conversationID uint, page, pageSize int) ([]*ConversationMessage, pagination.Page, error) {
	if _, err := uc.repo.FindConversation(ctx, userID, conversationID); err != nil {
		return nil, pagination.Page{}, err
	}
	list, total, err := uc.repo.ListMessages(ctx, conversationID, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// truncateRunes 按字符（非字节）截断，避免中文标题截出乱码
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// recordUsage 包装流通道：捕获 usage 帧并在流结束后落用量流水；
// 上游未回传 usage（调用失败/中断）时不记录
func (uc *Usecase) recordUsage(agentID, modelID, userID uint, start time.Time, in <-chan StreamEvent) <-chan StreamEvent {
	out := make(chan StreamEvent)
	go func() {
		defer close(out)
		var usage *Usage
		for ev := range in {
			if ev.Usage != nil {
				usage = ev.Usage
			}
			out <- ev
		}
		if usage == nil {
			return
		}
		if err := uc.repo.AppendUsage(context.Background(), &UsageLog{
			AgentID: agentID, ModelID: modelID, UserID: userID,
			PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
			TotalTokens: usage.TotalTokens, LatencyMs: time.Since(start).Milliseconds(),
		}); err != nil {
			logger.Warn("append usage log failed", zap.Uint("agent", agentID), zap.Error(err))
		}
	}()
	return out
}

// UsageStats 最近 days 日用量统计（Go 侧聚合，费用按模型当前单价估算）
func (uc *Usecase) UsageStats(ctx context.Context, days int) (*UsageStats, error) {
	if days <= 0 || days > 90 {
		days = 7
	}
	logs, err := uc.repo.ListUsageSince(ctx, time.Now().AddDate(0, 0, -(days - 1)).Truncate(24*time.Hour))
	if err != nil {
		return nil, err
	}
	models, _, err := uc.repo.ListModels(ctx, ModelQuery{}, 1, 0)
	if err != nil {
		return nil, err
	}
	priceByModel := make(map[uint]*Model, len(models))
	for _, m := range models {
		priceByModel[m.ID] = m
	}
	agents, _, err := uc.repo.ListAgents(ctx, AgentQuery{}, 1, 0)
	if err != nil {
		return nil, err
	}
	nameByAgent := make(map[uint]string, len(agents))
	for _, a := range agents {
		nameByAgent[a.ID] = a.Name
	}

	cost := func(modelID uint, prompt, completion int) float64 {
		m := priceByModel[modelID]
		if m == nil {
			return 0
		}
		return float64(prompt)/1000*m.InputPrice + float64(completion)/1000*m.OutputPrice
	}

	// 日期桶（含无调用日补零），本地时区
	buckets := make(map[string]*UsageDailyPoint, days)
	dates := make([]string, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		dates = append(dates, d)
		buckets[d] = &UsageDailyPoint{Date: d}
	}
	byAgent := make(map[uint]*UsageAgentPoint)
	stats := &UsageStats{Days: make([]UsageDailyPoint, 0, days)}
	for _, l := range logs {
		c := cost(l.ModelID, l.PromptTokens, l.CompletionTokens)
		if b, ok := buckets[l.CreatedAt.Format("2006-01-02")]; ok {
			b.Calls++
			b.PromptTokens += int64(l.PromptTokens)
			b.CompletionTokens += int64(l.CompletionTokens)
			b.TotalTokens += int64(l.TotalTokens)
			b.Cost += c
		}
		ap, ok := byAgent[l.AgentID]
		if !ok {
			ap = &UsageAgentPoint{AgentID: l.AgentID, AgentName: nameByAgent[l.AgentID]}
			byAgent[l.AgentID] = ap
		}
		ap.Calls++
		ap.TotalTokens += int64(l.TotalTokens)
		ap.Cost += c
		stats.Calls++
		stats.Tokens += int64(l.TotalTokens)
		stats.Cost += c
	}
	for _, d := range dates {
		stats.Days = append(stats.Days, *buckets[d])
	}
	for _, ap := range byAgent {
		stats.Agents = append(stats.Agents, *ap)
	}
	sort.Slice(stats.Agents, func(i, j int) bool {
		if stats.Agents[i].TotalTokens != stats.Agents[j].TotalTokens {
			return stats.Agents[i].TotalTokens > stats.Agents[j].TotalTokens
		}
		return stats.Agents[i].Calls > stats.Agents[j].Calls
	})
	if len(stats.Agents) > 10 {
		stats.Agents = stats.Agents[:10]
	}
	return stats, nil
}
