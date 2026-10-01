// Package mcp MCP 服务限界上下文 —— 领域层。
// 管理 MCP（Model Context Protocol）服务器配置（远程 streamable HTTP / SSE 传输），
// 自研 JSON-RPC 2.0 客户端实现连通测试与工具发现/调用；
// 智能体经 mcp:<serverCode>:<toolName> 引用绑定 MCP 工具参与 function calling。
package mcp

import (
	"context"
	"errors"
	"time"
)

// 哨兵错误
var (
	ErrNotFound         = errors.New("MCP 服务不存在")
	ErrCodeExists       = errors.New("MCP 服务编码已存在")
	ErrNameExists       = errors.New("MCP 服务名称已存在")
	ErrInvalidCode      = errors.New("MCP 服务编码仅限字母数字开头，可含下划线与中划线")
	ErrInvalidTransport = errors.New("不支持的 MCP 传输类型")
	ErrInvalidURL       = errors.New("MCP 服务地址必须为合法 http(s) URL")
	ErrServerInUse      = errors.New("MCP 服务已被智能体绑定，请先解除引用")
	ErrServerDisabled   = errors.New("MCP 服务已禁用")
	ErrConnectFailed    = errors.New("MCP 服务连接失败")
	ErrTimeout          = errors.New("MCP 服务响应超时")
	ErrDecryptFailed    = errors.New("MCP 凭证解密失败，请重新保存 Token")
	ErrProtocol         = errors.New("MCP 服务返回了无法解析的响应")
)

// Transport 传输类型
type Transport string

const (
	// TransportStreamableHTTP 2025-03-26 规范：单端点 POST，响应 JSON 或 SSE 流
	TransportStreamableHTTP Transport = "streamable_http"
	// TransportSSE 旧版 2024-11-05：GET 长连接接收 endpoint 事件，消息走独立 POST 端点
	TransportSSE Transport = "sse"
)

func ValidTransport(t string) bool {
	return t == string(TransportStreamableHTTP) || t == string(TransportSSE)
}

// Header 自定义请求头（明文存储的非敏感头；敏感凭证走 Token 加密通道）
type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Server MCP 服务器配置（json tag 与前端类型字段对齐）
type Server struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"` // 稳定引用（Agent.Tools 以 mcp:<code>:<tool> 引用）
	Transport Transport `json:"transport"`
	BaseURL   string    `json:"base_url"` // MCP 端点（streamable: 消息端点；sse: 事件流端点）
	Headers   []Header  `json:"headers"`
	// TokenEnc AES-GCM 密文（base64），永不明文出域；展示用掩码见 TokenMask；发送时作 Authorization: Bearer
	TokenEnc  string    `json:"-"`
	TokenMask string    `json:"token_mask"`
	Remark    string    `json:"remark"`
	Status    int       `json:"status"` // 1 启用 0 禁用
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToolInfo MCP 工具元信息（tools/list 返回）
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"` // JSON Schema，直接映射 OpenAI function parameters
}

// Query 管理端列表条件
type Query struct {
	Kw        string // 名称/编码模糊
	Transport string
	Status    *int
}

// Repo 仓储接口
type Repo interface {
	Create(ctx context.Context, s *Server) error
	Update(ctx context.Context, s *Server) error
	// Delete 软删并归档唯一列（name/code）
	Delete(ctx context.Context, id uint) error
	Find(ctx context.Context, id uint) (*Server, error)
	FindByCode(ctx context.Context, code string) (*Server, error)
	FindByName(ctx context.Context, name string) (*Server, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*Server, int64, error)
	// ListEnabled 全部启用配置（工具发现/会话管理用）
	ListEnabled(ctx context.Context) ([]*Server, error)
	// CountAgentsUsing Agent 绑定预检：引用 mcp:<code>: 的智能体数（agents.tools JSON 模糊匹配）
	CountAgentsUsing(ctx context.Context, code string) (int64, error)
}
