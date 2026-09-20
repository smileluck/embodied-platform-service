// 工具注册表：LLM function calling 的服务端执行体集合。
// 内置只读示范工具见 builtin.go；业务模块后续经 Register 扩展自有工具。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// ToolCall 上游返回的工具调用请求（StreamCompletion 聚合后完整下发）
type ToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // 原始 JSON 字符串（上游分片拼接结果）
	} `json:"function"`
}

// ToolCallResult 工具执行结果（SSE tool 帧下发，前端折叠卡片展示）
type ToolCallResult struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"` // 执行失败时的原因（仍回传给模型自查）
}

// ToolDef 下发给上游的工具定义（OpenAI functions 格式）
type ToolDef struct {
	Type     string   `json:"type"` // 固定 "function"
	Function ToolFunc `json:"function"`
}

// ToolFunc 工具元信息（Parameters 为 JSON Schema 对象）
type ToolFunc struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Tool 本地工具：声明定义 + 执行体
type Tool interface {
	Def() ToolFunc
	// Execute 执行工具调用；args 为上游给出的参数 JSON 字符串
	Execute(ctx context.Context, args string) (string, error)
}

// ToolRegistry 工具注册表（构造后只读）
type ToolRegistry struct {
	tools map[string]Tool
	names []string
}

func NewToolRegistry(tools ...Tool) *ToolRegistry {
	r := &ToolRegistry{tools: make(map[string]Tool, len(tools))}
	for _, t := range tools {
		name := t.Def().Name
		if name == "" {
			continue
		}
		if _, dup := r.tools[name]; dup {
			continue
		}
		r.tools[name] = t
		r.names = append(r.names, name)
	}
	sort.Strings(r.names)
	return r
}

// Get 按名取工具
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Names 全部已注册工具名（排序稳定）
func (r *ToolRegistry) Names() []string { return r.names }

// Defs 全部工具的下发定义
func (r *ToolRegistry) Defs() []ToolDef {
	out := make([]ToolDef, 0, len(r.names))
	for _, n := range r.names {
		out = append(out, ToolDef{Type: "function", Function: r.tools[n].Def()})
	}
	return out
}

// funcTool 函数式工具实现（内置小工具用，避免每个工具一个结构体）
type funcTool struct {
	def  ToolFunc
	exec func(ctx context.Context, args string) (string, error)
}

func (f *funcTool) Def() ToolFunc { return f.def }

func (f *funcTool) Execute(ctx context.Context, args string) (string, error) {
	return f.exec(ctx, args)
}

// simpleJSONArg 解析无参数工具的 args（容忍空串与 {}）
func simpleJSONArg(args string) error {
	s := trimSpace(args)
	if s == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return fmt.Errorf("参数须为 JSON 对象: %w", err)
	}
	return nil
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
