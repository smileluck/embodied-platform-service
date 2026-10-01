package agent

// MCP 工具链接入单测：mcp:<code>:<tool> 引用的校验、下发定义（safeName）、执行路由与回显还原。

import (
	"context"
	"errors"
	"testing"

	bizmcp "github.com/smilex/smilex-admin-gin/internal/biz/mcp"
	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// fakeMCPSource 内存 MCP 工具源（server “demo” 启用，提供 echo 工具）
type fakeMCPSource struct {
	disabled map[string]bool
	tools    map[string][]bizmcp.ToolInfo
	calls    []string // 已执行的 server:tool
}

func newFakeMCP() *fakeMCPSource {
	return &fakeMCPSource{
		disabled: map[string]bool{},
		tools: map[string][]bizmcp.ToolInfo{
			"demo": {{Name: "echo", Description: "回声", InputSchema: map[string]any{"type": "object"}}},
		},
	}
}

func (f *fakeMCPSource) ToolDefs(_ context.Context, code string) ([]bizmcp.ToolInfo, error) {
	if tools, ok := f.tools[code]; ok {
		return tools, nil
	}
	return nil, bizmcp.ErrNotFound
}

func (f *fakeMCPSource) EnabledServers(context.Context) ([]bizmcp.ServerRef, error) {
	return []bizmcp.ServerRef{{ID: 1, Code: "demo", Name: "演示服务"}}, nil
}

func (f *fakeMCPSource) CheckServer(_ context.Context, code string) error {
	if _, ok := f.tools[code]; !ok {
		return bizmcp.ErrNotFound
	}
	if f.disabled[code] {
		return bizmcp.ErrServerDisabled
	}
	return nil
}

func (f *fakeMCPSource) CallTool(_ context.Context, code, tool, _ string) (string, error) {
	f.calls = append(f.calls, code+":"+tool)
	if code == "demo" && tool == "echo" {
		return "pong", nil
	}
	return "", errors.New("no such tool")
}

func newUsecaseWithMCP(repo Repo, mcp MCPToolSource) *Usecase {
	return NewUsecase(repo, &conf.Bootstrap{
		Agent: conf.Agent{CryptoKey: "unit-test-key"},
		JWT:   conf.JWT{Secret: "jwt-secret"},
	}, nil, mcp, nil)
}

func TestParseMCPRef(t *testing.T) {
	if _, ok := parseMCPRef("now"); ok {
		t.Fatal("plain name should not parse")
	}
	ref, ok := parseMCPRef("mcp:demo:echo")
	if !ok || ref.serverCode != "demo" || ref.toolName != "echo" || ref.origin != "mcp:demo:echo" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
	for _, bad := range []string{"mcp:", "mcp:x:", "mcp::y", "mcp:x"} {
		if _, ok := parseMCPRef(bad); ok {
			t.Fatalf("%q should not parse", bad)
		}
	}
}

func TestMCPValidateTools(t *testing.T) {
	uc := newUsecaseWithMCP(newFakeRepo(), newFakeMCP())
	ctx := context.Background()

	// 合法混合绑定：本地 + MCP（去重）
	got, err := uc.validateTools(ctx, []string{"now", "mcp:demo:echo", "now", "mcp:demo:echo"})
	if err != nil || len(got) != 2 {
		t.Fatalf("validateTools: %v %v", got, err)
	}
	// 服务不存在
	if _, err := uc.validateTools(ctx, []string{"mcp:ghost:x"}); !errors.Is(err, bizmcp.ErrNotFound) {
		t.Fatalf("ghost server should fail with ErrNotFound, got %v", err)
	}
	// 本地未知工具仍报 ErrUnknownTool
	if _, err := uc.validateTools(ctx, []string{"nope"}); !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("unknown local tool: %v", err)
	}
}

func TestMCPToolDefsAndExec(t *testing.T) {
	fake := newFakeMCP()
	uc := newUsecaseWithMCP(newFakeRepo(), fake)
	a := &Agent{Tools: []string{"now", "mcp:demo:echo"}}
	m := &Model{SupportsTools: true}

	defs, resolver := uc.toolDefsFor(context.Background(), a, m)
	if len(defs) != 2 {
		t.Fatalf("expect 2 defs (local + mcp), got %d", len(defs))
	}
	var echoDef *ToolDef
	for i, d := range defs {
		if d.Function.Name == "mcp__demo__echo" {
			echoDef = &defs[i]
		}
	}
	if echoDef == nil {
		t.Fatalf("mcp def missing: %+v", defs)
	}
	if echoDef.Function.Description != "回声" || echoDef.Function.Parameters["type"] != "object" {
		t.Fatalf("unexpected mcp def: %+v", echoDef.Function)
	}
	ref, ok := resolver["mcp__demo__echo"]
	if !ok || ref.toolName != "echo" {
		t.Fatalf("resolver missing: %+v", resolver)
	}

	// 执行路由：safeName 入参 -> MCP 调用 -> 回显 origin
	call := ToolCall{ID: "1"}
	call.Function.Name = "mcp__demo__echo"
	call.Function.Arguments = `{"text":"ping"}`
	res := uc.execTool(call, resolver)
	if res.Error != "" || res.Result != "pong" {
		t.Fatalf("exec mcp tool: %+v", res)
	}
	if res.Name != "mcp:demo:echo" {
		t.Fatalf("result name should restore origin, got %q", res.Name)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "demo:echo" {
		t.Fatalf("unexpected mcp calls: %v", fake.calls)
	}

	// 未知函数名：报 unknown tool
	bad := ToolCall{ID: "2"}
	bad.Function.Name = "mcp__demo__missing"
	if r := uc.execTool(bad, resolver); r.Error == "" {
		t.Fatal("missing tool should error")
	}
}

func TestMCPToolDefsUnavailable(t *testing.T) {
	// 服务不可达（ToolDefs 报错）：跳过该服务，本地工具不受影响
	uc := newUsecaseWithMCP(newFakeRepo(), &brokenMCPSource{})
	a := &Agent{Tools: []string{"now", "mcp:demo:echo"}}
	defs, resolver := uc.toolDefsFor(context.Background(), a, &Model{SupportsTools: true})
	if len(defs) != 1 || defs[0].Function.Name != "now" {
		t.Fatalf("expect only local def, got %+v", defs)
	}
	if len(resolver) != 0 {
		t.Fatalf("resolver should be empty, got %+v", resolver)
	}
}

type brokenMCPSource struct{}

func (brokenMCPSource) ToolDefs(context.Context, string) ([]bizmcp.ToolInfo, error) {
	return nil, bizmcp.ErrConnectFailed
}
func (brokenMCPSource) EnabledServers(context.Context) ([]bizmcp.ServerRef, error) {
	return nil, nil
}
func (brokenMCPSource) CheckServer(context.Context, string) error { return nil }
func (brokenMCPSource) CallTool(context.Context, string, string, string) (string, error) {
	return "", bizmcp.ErrConnectFailed
}
