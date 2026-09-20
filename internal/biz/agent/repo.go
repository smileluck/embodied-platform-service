package agent

import (
	"context"
	"errors"
)

var (
	// ErrProviderNotFound 供应商不存在
	ErrProviderNotFound = errors.New("LLM 供应商不存在")
	// ErrProviderCodeExists 供应商编码已存在
	ErrProviderCodeExists = errors.New("供应商编码已存在，请更换")
	// ErrProviderHasModels 供应商下仍有模型，须先删除
	ErrProviderHasModels = errors.New("该供应商下存在模型，请先删除模型")
	// ErrProviderNoModels 供应商下没有启用的模型可供测试
	ErrProviderNoModels = errors.New("该供应商下没有启用的模型，请先添加")
	// ErrProviderDisabled 供应商已被禁用
	ErrProviderDisabled = errors.New("该供应商已被禁用")

	// ErrModelNotFound 模型不存在
	ErrModelNotFound = errors.New("模型不存在")
	// ErrModelExists 同一供应商下已存在同名模型
	ErrModelExists = errors.New("该供应商下已存在同名模型，请更换")
	// ErrModelInUse 模型已被 Agent 引用
	ErrModelInUse = errors.New("该模型已被 Agent 引用，请先调整 Agent 配置")
	// ErrModelDisabled 模型已被禁用
	ErrModelDisabled = errors.New("该模型已被禁用")

	// ErrAgentNotFound Agent 不存在
	ErrAgentNotFound = errors.New("Agent 不存在")
	// ErrAgentCodeExists Agent 编码已存在
	ErrAgentCodeExists = errors.New("Agent 编码已存在，请更换")
	// ErrAgentDisabled Agent 已被禁用
	ErrAgentDisabled = errors.New("该 Agent 已被禁用")

	// ErrDecryptFailed API Key 解密失败（加密密钥可能已变更）
	ErrDecryptFailed = errors.New("API Key 解密失败，加密密钥可能已变更，请重新保存密钥")

	// ErrLLMUpstream LLM 上游调用失败（携带上游返回的具体原因）
	ErrLLMUpstream = errors.New("LLM 上游调用失败")
	// ErrLLMTimeout LLM 上游调用超时
	ErrLLMTimeout = errors.New("LLM 上游调用超时，请稍后重试")
)

// ProviderQuery 供应商列表查询条件
type ProviderQuery struct {
	Name   string
	Code   string
	Status *int
}

// ModelQuery 模型列表查询条件
type ModelQuery struct {
	ProviderID *uint
	Name       string
	Status     *int
}

// AgentQuery Agent 列表查询条件
type AgentQuery struct {
	Name   string
	Code   string
	Status *int
}

// Repo 智能体仓储接口
type Repo interface {
	CreateProvider(ctx context.Context, p *Provider) error
	UpdateProvider(ctx context.Context, p *Provider) error
	DeleteProvider(ctx context.Context, id uint) error
	FindProviderByID(ctx context.Context, id uint) (*Provider, error)
	FindProviderByCode(ctx context.Context, code string) (*Provider, error)
	ListProviders(ctx context.Context, q ProviderQuery, page, pageSize int) ([]*Provider, int64, error)
	// CountModelsByProvider 统计供应商下的模型数量（删除保护用）
	CountModelsByProvider(ctx context.Context, providerID uint) (int64, error)

	CreateModel(ctx context.Context, m *Model) error
	UpdateModel(ctx context.Context, m *Model) error
	DeleteModel(ctx context.Context, id uint) error
	FindModelByID(ctx context.Context, id uint) (*Model, error)
	FindModelByName(ctx context.Context, providerID uint, name string) (*Model, error)
	// FindFirstEnabledModel 取供应商下首个启用模型（供应商连通性测试默认模型）
	FindFirstEnabledModel(ctx context.Context, providerID uint) (*Model, error)
	ListModels(ctx context.Context, q ModelQuery, page, pageSize int) ([]*Model, int64, error)
	// CountAgentsByModel 统计引用某模型的 Agent 数量（删除保护用）
	CountAgentsByModel(ctx context.Context, modelID uint) (int64, error)

	CreateAgent(ctx context.Context, a *Agent) error
	UpdateAgent(ctx context.Context, a *Agent) error
	DeleteAgent(ctx context.Context, id uint) error
	FindAgentByID(ctx context.Context, id uint) (*Agent, error)
	// FindAgentByCode 业务模块按 code 引用 Agent 配置的底座入口
	FindAgentByCode(ctx context.Context, code string) (*Agent, error)
	ListAgents(ctx context.Context, q AgentQuery, page, pageSize int) ([]*Agent, int64, error)
}
