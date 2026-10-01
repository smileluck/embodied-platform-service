package agent

// 智能体绑定 MCP 工具：Agent.Tools 支持 mcp:<serverCode>:<toolName> 引用（库存可读格式）。
// 下发上游时转为合法 function 名 mcp__<code>__<tool>（OpenAI 函数名不允许冒号），
// 执行与回显经 resolver 映射表还原，不依赖字符串拆解（规避 code/tool 含分隔符的歧义）。

import (
	"context"
	"regexp"
	"strings"

	bizmcp "github.com/smilex/smilex-admin-gin/internal/biz/mcp"
)

// mcpToolPrefix MCP 工具引用前缀（Agent.Tools 库存格式）
const mcpToolPrefix = "mcp:"

// MCPToolSource MCP 工具源（跨上下文最小接口，实现见 mcp 限界上下文的 Usecase）
type MCPToolSource interface {
	// ToolDefs 指定服务的工具定义（tools/list，远端拉取带短缓存）
	ToolDefs(ctx context.Context, serverCode string) ([]bizmcp.ToolInfo, error)
	// EnabledServers 全部启用的 MCP 服务（Agent 表单工具分组数据源）
	EnabledServers(ctx context.Context) ([]bizmcp.ServerRef, error)
	// CheckServer 校验编码存在且启用（Agent 绑定引用时强校验；工具存在性动态，不强校验）
	CheckServer(ctx context.Context, serverCode string) error
	// CallTool 执行远程工具（结果为拼接文本）
	CallTool(ctx context.Context, serverCode, tool, argsJSON string) (string, error)
}

// mcpToolRef MCP 工具引用解析结果
type mcpToolRef struct {
	origin     string // Agent.Tools 中的原始引用（mcp:<code>:<tool>），结果回显用
	serverCode string
	toolName   string
}

// parseMCPRef 解析 mcp:<code>:<tool>；非该前缀或格式非法返回 ok=false
func parseMCPRef(name string) (mcpToolRef, bool) {
	if !strings.HasPrefix(name, mcpToolPrefix) {
		return mcpToolRef{}, false
	}
	rest := name[len(mcpToolPrefix):]
	idx := strings.Index(rest, ":")
	if idx <= 0 || idx == len(rest)-1 {
		return mcpToolRef{}, false
	}
	return mcpToolRef{origin: name, serverCode: rest[:idx], toolName: rest[idx+1:]}, true
}

// unsafeFuncChar OpenAI function name 合法字符集之外的字符（替换为下划线）
var unsafeFuncChar = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// safeName 下发给上游的合法 function 名（^[a-zA-Z0-9_-]{1,64}$，超长截断）
func (r mcpToolRef) safeName() string {
	n := "mcp__" + unsafeFuncChar.ReplaceAllString(r.serverCode, "_") + "__" + unsafeFuncChar.ReplaceAllString(r.toolName, "_")
	if len(n) > 64 {
		n = n[:64]
	}
	return n
}

// mcpResolver 下发函数名 -> 工具引用（chat 会话内构建，执行与回显反查）
type mcpResolver map[string]mcpToolRef
