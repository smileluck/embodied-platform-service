// Package agent 智能体应用服务（LLM 配置底座）
package agent

import (
	"context"

	bizagent "github.com/smilex/smilex-admin-gin/internal/biz/agent"
)

type Service struct {
	uc *bizagent.Usecase
}

func NewService(uc *bizagent.Usecase) *Service { return &Service{uc: uc} }

// ---- 供应商 ----

type ProviderCreateRequest struct {
	Name    string `json:"name" binding:"required,max=20"`
	Code    string `json:"code" binding:"required,max=64"`
	BaseURL string `json:"base_url" binding:"required,max=255"`
	// APIKey 明文仅写入链路；留空表示不配置（本地服务如 Ollama）；更新时留空=保持不变
	APIKey string `json:"api_key" binding:"max=512"`
	Remark string `json:"remark" binding:"max=200"`
	Status *int   `json:"status" binding:"omitempty,gte=0,lte=1"` // 缺省启用
}

type ProviderUpdateRequest struct {
	Name    string `json:"name" binding:"omitempty,max=20"` // 空=保持原名
	Code    string `json:"code" binding:"omitempty,max=64"`
	BaseURL string `json:"base_url" binding:"omitempty,max=255"`
	APIKey  string `json:"api_key" binding:"max=512"` // 空=保持原密钥
	Remark  string `json:"remark" binding:"max=200"`
	Status  *int   `json:"status" binding:"omitempty,gte=0,lte=1"`
}

// TestRequest 连通性测试入参（model_id 为 0 时取供应商首个启用模型）
type TestRequest struct {
	ModelID uint `json:"model_id" binding:"omitempty,gt=0"`
}

func (s *Service) CreateProvider(ctx context.Context, req ProviderCreateRequest) (*bizagent.Provider, error) {
	return s.uc.CreateProvider(ctx, bizagent.ProviderInput{
		Name: req.Name, Code: req.Code, BaseURL: req.BaseURL, APIKey: req.APIKey,
		Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) UpdateProvider(ctx context.Context, id uint, req ProviderUpdateRequest) error {
	return s.uc.UpdateProvider(ctx, id, bizagent.ProviderInput{
		Name: req.Name, Code: req.Code, BaseURL: req.BaseURL, APIKey: req.APIKey,
		Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) DeleteProvider(ctx context.Context, id uint) error {
	return s.uc.DeleteProvider(ctx, id)
}
func (s *Service) GetProvider(ctx context.Context, id uint) (*bizagent.Provider, error) {
	return s.uc.GetProvider(ctx, id)
}

func (s *Service) ListProviders(ctx context.Context, q bizagent.ProviderQuery, page, pageSize int) ([]*bizagent.Provider, interface{}, error) {
	return s.uc.ListProviders(ctx, q, page, pageSize)
}

func (s *Service) TestProvider(ctx context.Context, providerID uint, req TestRequest) (*bizagent.TestResult, error) {
	return s.uc.TestProvider(ctx, providerID, req.ModelID)
}

func (s *Service) RemoteModels(ctx context.Context, providerID uint) ([]string, error) {
	return s.uc.RemoteModels(ctx, providerID)
}

// ---- 模型 ----

type ModelCreateRequest struct {
	ProviderID    uint   `json:"provider_id" binding:"required,gt=0"`
	Name          string `json:"name" binding:"required,max=128"`
	DisplayName   string `json:"display_name" binding:"max=20"`
	ContextWindow int    `json:"context_window" binding:"gte=0,lte=100000000"`
	MaxOutput     int    `json:"max_output" binding:"gte=0,lte=100000000"`
	SupportsTools bool   `json:"supports_tools"`
	Remark        string `json:"remark" binding:"max=200"`
	Status        *int   `json:"status" binding:"omitempty,gte=0,lte=1"`
}

type ModelUpdateRequest struct {
	ProviderID    uint   `json:"provider_id" binding:"omitempty,gt=0"` // 空=保持原供应商
	Name          string `json:"name" binding:"omitempty,max=128"`
	DisplayName   string `json:"display_name" binding:"max=20"`
	ContextWindow int    `json:"context_window" binding:"gte=0,lte=100000000"`
	MaxOutput     int    `json:"max_output" binding:"gte=0,lte=100000000"`
	SupportsTools bool   `json:"supports_tools"`
	Remark        string `json:"remark" binding:"max=200"`
	Status        *int   `json:"status" binding:"omitempty,gte=0,lte=1"`
}

func (s *Service) CreateModel(ctx context.Context, req ModelCreateRequest) (*bizagent.Model, error) {
	return s.uc.CreateModel(ctx, bizagent.ModelInput{
		ProviderID: req.ProviderID, Name: req.Name, DisplayName: req.DisplayName,
		ContextWindow: req.ContextWindow, MaxOutput: req.MaxOutput, SupportsTools: req.SupportsTools,
		Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) UpdateModel(ctx context.Context, id uint, req ModelUpdateRequest) error {
	return s.uc.UpdateModel(ctx, id, bizagent.ModelInput{
		ProviderID: req.ProviderID, Name: req.Name, DisplayName: req.DisplayName,
		ContextWindow: req.ContextWindow, MaxOutput: req.MaxOutput, SupportsTools: req.SupportsTools,
		Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) DeleteModel(ctx context.Context, id uint) error { return s.uc.DeleteModel(ctx, id) }
func (s *Service) GetModel(ctx context.Context, id uint) (*bizagent.Model, error) {
	return s.uc.GetModel(ctx, id)
}

func (s *Service) ListModels(ctx context.Context, q bizagent.ModelQuery, page, pageSize int) ([]*bizagent.Model, interface{}, error) {
	return s.uc.ListModels(ctx, q, page, pageSize)
}

func (s *Service) TestModel(ctx context.Context, modelID uint) (*bizagent.TestResult, error) {
	return s.uc.TestModel(ctx, modelID)
}

// ---- Agent ----

type AgentCreateRequest struct {
	Name         string  `json:"name" binding:"required,max=20"`
	Code         string  `json:"code" binding:"required,max=64"`
	ModelID      uint    `json:"model_id" binding:"required,gt=0"`
	SystemPrompt string  `json:"system_prompt" binding:"max=4000"`
	Temperature  float64 `json:"temperature" binding:"gte=0,lte=2"`
	TopP         float64 `json:"top_p" binding:"gte=0,lte=1"`
	MaxTokens    int     `json:"max_tokens" binding:"gte=0,lte=131072"` // 0=上游默认
	Remark       string  `json:"remark" binding:"max=200"`
	Status       *int    `json:"status" binding:"omitempty,gte=0,lte=1"`
}

type AgentUpdateRequest struct {
	Name         string  `json:"name" binding:"omitempty,max=20"` // 空=保持原名
	Code         string  `json:"code" binding:"omitempty,max=64"`
	ModelID      uint    `json:"model_id" binding:"omitempty,gt=0"` // 空=保持原模型
	SystemPrompt string  `json:"system_prompt" binding:"max=4000"`
	Temperature  float64 `json:"temperature" binding:"gte=0,lte=2"`
	TopP         float64 `json:"top_p" binding:"gte=0,lte=1"`
	MaxTokens    int     `json:"max_tokens" binding:"gte=0,lte=131072"`
	Remark       string  `json:"remark" binding:"max=200"`
	Status       *int    `json:"status" binding:"omitempty,gte=0,lte=1"`
}

func (s *Service) CreateAgent(ctx context.Context, req AgentCreateRequest) (*bizagent.Agent, error) {
	return s.uc.CreateAgent(ctx, bizagent.AgentInput{
		Name: req.Name, Code: req.Code, ModelID: req.ModelID,
		SystemPrompt: req.SystemPrompt, Temperature: req.Temperature, TopP: req.TopP,
		MaxTokens: req.MaxTokens, Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) UpdateAgent(ctx context.Context, id uint, req AgentUpdateRequest) error {
	return s.uc.UpdateAgent(ctx, id, bizagent.AgentInput{
		Name: req.Name, Code: req.Code, ModelID: req.ModelID,
		SystemPrompt: req.SystemPrompt, Temperature: req.Temperature, TopP: req.TopP,
		MaxTokens: req.MaxTokens, Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) DeleteAgent(ctx context.Context, id uint) error { return s.uc.DeleteAgent(ctx, id) }
func (s *Service) GetAgent(ctx context.Context, id uint) (*bizagent.Agent, error) {
	return s.uc.GetAgent(ctx, id)
}

func (s *Service) ListAgents(ctx context.Context, q bizagent.AgentQuery, page, pageSize int) ([]*bizagent.Agent, interface{}, error) {
	return s.uc.ListAgents(ctx, q, page, pageSize)
}

// ---- 调试对话（Playground） ----

// ChatMessage 调试对话消息（system 由 Agent 配置注入，前端仅可传 user/assistant）
type ChatMessage struct {
	Role    string `json:"role" binding:"required,oneof=user assistant"`
	Content string `json:"content" binding:"required,max=4000"`
}

type ChatRequest struct {
	// Messages 对话历史（末条须为 user）；最后一条 user 消息之后的 assistant 增量由 SSE 流返回
	Messages []ChatMessage `json:"messages" binding:"required,min=1,max=50,dive"`
}

func (s *Service) ChatStream(ctx context.Context, agentID uint, req ChatRequest) (<-chan bizagent.StreamEvent, *bizagent.ChatMeta, error) {
	msgs := make([]bizagent.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, bizagent.Message{Role: m.Role, Content: m.Content})
	}
	return s.uc.ChatStream(ctx, agentID, msgs)
}

// statusOf 状态缺省启用（未传时默认 1）
func statusOf(s *int) bizagent.Status {
	if s != nil && *s == 0 {
		return bizagent.StatusDisabled
	}
	return bizagent.StatusEnabled
}

// ---- 会话（本人数据） ----

// ConversationCreateRequest 新建会话入参
type ConversationCreateRequest struct {
	AgentID uint `json:"agent_id" binding:"required,gt=0"`
}

// ConversationRenameRequest 会话改名入参
type ConversationRenameRequest struct {
	Title string `json:"title" binding:"required,max=20"`
}

func (s *Service) CreateConversation(ctx context.Context, userID uint, req ConversationCreateRequest) (*bizagent.Conversation, error) {
	return s.uc.CreateConversation(ctx, userID, req.AgentID)
}

func (s *Service) RenameConversation(ctx context.Context, userID, id uint, req ConversationRenameRequest) (*bizagent.Conversation, error) {
	return s.uc.RenameConversation(ctx, userID, id, req.Title)
}

func (s *Service) DeleteConversation(ctx context.Context, userID, id uint) error {
	return s.uc.DeleteConversation(ctx, userID, id)
}

func (s *Service) ListConversations(ctx context.Context, userID uint, agentID *uint, page, pageSize int) ([]*bizagent.Conversation, interface{}, error) {
	q := bizagent.ConversationQuery{AgentID: agentID}
	return s.uc.ListConversations(ctx, userID, q, page, pageSize)
}

func (s *Service) ListConversationMessages(ctx context.Context, userID, conversationID uint, page, pageSize int) ([]*bizagent.ConversationMessage, interface{}, error) {
	return s.uc.ListConversationMessages(ctx, userID, conversationID, page, pageSize)
}
