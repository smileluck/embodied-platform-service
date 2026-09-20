// Package agent 智能体限界上下文 —— 领域层。
// harness 底座：三层配置（供应商 -> 模型 -> Agent）+ LLM 调用运行时，
// 业务模块按 Agent.Code 稳定引用，后续适配不同业务时在此层扩展工具/记忆等能力。
package agent

import "time"

// Status 通用状态值对象（供应商/模型/Agent 共用）
type Status int

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

// ProtocolOpenAI OpenAI Chat Completions 兼容协议（本期唯一实现，字段预留多协议扩展）
const ProtocolOpenAI = "openai"

// Provider LLM 供应商配置（json tag 与前端类型字段对齐）
type Provider struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Code    string `json:"code"`
	BaseURL string `json:"base_url"` // 如 https://open.bigmodel.cn/api/paas/v4
	// APIKeyEnc AES-GCM 密文（base64），永不明文出域；展示用掩码见 APIKeyMask
	APIKeyEnc  string    `json:"-"`
	APIKeyMask string    `json:"api_key_mask"` // 展示掩码（如 sk-****abcd；未配置为空）
	Protocol   string    `json:"protocol"`
	Remark     string    `json:"remark"`
	Status     Status    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// HasAPIKey 是否已配置密钥
func (p *Provider) HasAPIKey() bool { return p.APIKeyEnc != "" }

// Model 供应商下的模型配置
type Model struct {
	ID            uint      `json:"id"`
	ProviderID    uint      `json:"provider_id"`
	Name          string    `json:"name"`           // 上游模型标识（如 glm-4.7）
	DisplayName   string    `json:"display_name"`   // 展示名（空则回退 name）
	ContextWindow int       `json:"context_window"` // 上下文窗口（token，0=未知）
	MaxOutput     int       `json:"max_output"`     // 单次最大输出（token，0=上游默认）
	SupportsTools bool      `json:"supports_tools"` // 是否支持工具调用（function call）
	// 每千 token 计费单价（货币单位由运营口径自定，仅用于费用估算展示）；0=未设置不估算
	InputPrice  float64   `json:"input_price"`
	OutputPrice float64   `json:"output_price"`
	Remark      string    `json:"remark"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Display 展示名（未设置时回退模型标识）
func (m *Model) Display() string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	return m.Name
}

// Agent 智能体配置（业务按 Code 稳定引用）
type Agent struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	Code         string    `json:"code"`
	ModelID      uint      `json:"model_id"`
	SystemPrompt string    `json:"system_prompt"`
	Temperature  float64   `json:"temperature"` // 0~2；0 表示未设置（用上游默认）
	TopP         float64   `json:"top_p"`       // 0~1；0 表示未设置
	MaxTokens    int       `json:"max_tokens"`  // 0=上游默认
	Remark       string    `json:"remark"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
