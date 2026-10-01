package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReadRPCFromSSE(t *testing.T) {
	body := strings.Join([]string{
		`: heartbeat comment`,
		`event: message`,
		`data: {"jsonrpc":"2.0","id":99,"result":{"noop":true}}`,
		``,
		`data: {"jsonrpc":"2.0","id":7,"result":{"tools":[]}}`,
		``,
		``,
	}, "\n")
	resp, err := readRPCFromSSE(strings.NewReader(body), 7)
	if err != nil {
		t.Fatalf("readRPCFromSSE: %v", err)
	}
	var got toolsListResult
	if err := json.Unmarshal(resp.Result, &got); err != nil || len(got.Tools) != 0 {
		t.Fatalf("unexpected result: %v %v", resp.Result, err)
	}
	// 目标 id 不存在 -> ErrProtocol
	if _, err := readRPCFromSSE(strings.NewReader(body), 8); err == nil {
		t.Fatal("expect error for missing id")
	}
}

func TestResolveEndpoint(t *testing.T) {
	cases := []struct{ base, ep, want string }{
		{"https://h/mcp", "/messages?sessionId=1", "https://h/messages?sessionId=1"},
		{"https://h/a/sse", "messages?x=1", "https://h/a/messages?x=1"},
		{"https://h/a/sse", "https://other/msg", "https://other/msg"},
	}
	for _, c := range cases {
		got, err := resolveEndpoint(c.base, c.ep)
		if err != nil || got != c.want {
			t.Fatalf("resolveEndpoint(%q,%q) = %q,%v want %q", c.base, c.ep, got, err, c.want)
		}
	}
	if _, err := resolveEndpoint("https://h/sse", "  "); err == nil {
		t.Fatal("empty endpoint should error")
	}
}

func TestIsProtectedHeader(t *testing.T) {
	for _, k := range []string{"Authorization", "authorization", "Content-Type", "mcp-session-id", "Cookie"} {
		if !isProtectedHeader(k) {
			t.Fatalf("%s should be protected", k)
		}
	}
	if isProtectedHeader("X-Custom-Trace") {
		t.Fatal("custom header should not be protected")
	}
}

// newStreamableServer 模拟 streamable HTTP MCP 服务器（JSON 响应体）
func newStreamableServer(t *testing.T, bearer string) *httptest.Server {
	t.Helper()
	var session string
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if bearer != "" && r.Header.Get("Authorization") != "Bearer "+bearer {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if sid := r.Header.Get("Mcp-Session-Id"); sid == "" || sid != session {
			// 新会话握手
			mu.Lock()
			session = fmt.Sprintf("sess-%d", time.Now().UnixNano())
			mu.Unlock()
			w.Header().Set("Mcp-Session-Id", session)
		}
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID,
				"result": map[string]any{
					"protocolVersion": protocolVersion,
					"serverInfo":      map[string]string{"name": "mock", "version": "9.9"},
				},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID,
				"result": map[string]any{"tools": []map[string]any{
					{"name": "echo", "description": "回声", "inputSchema": map[string]any{"type": "object"}},
				}},
			})
		case "tools/call":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID,
				"result": map[string]any{"content": []map[string]any{{"type": "text", "text": "pong"}}},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "method not found"},
			})
		}
	})
	return httptest.NewServer(mux)
}

func TestStreamableClient(t *testing.T) {
	srv := newStreamableServer(t, "tok-123")
	defer srv.Close()

	cli := NewClient(ClientConfig{Transport: TransportStreamableHTTP, BaseURL: srv.URL + "/mcp", Token: "tok-123"})
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, proto, err := cli.Handshake(ctx)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	if info.Name != "mock" || info.Version != "9.9" || proto != protocolVersion {
		t.Fatalf("unexpected server info: %+v %s", info, proto)
	}
	tools, err := cli.ListTools(ctx)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" || tools[0].InputSchema["type"] != "object" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	out, err := cli.CallTool(ctx, "echo", `{"text":"ping"}`)
	if err != nil || out != "pong" {
		t.Fatalf("call tool: %q %v", out, err)
	}
	// 错误凭证：握手应报连接失败
	bad := NewClient(ClientConfig{Transport: TransportStreamableHTTP, BaseURL: srv.URL + "/mcp", Token: "wrong"})
	defer bad.Close()
	if _, _, err := bad.Handshake(ctx); err == nil {
		t.Fatal("bad token should fail")
	}
}

// TestSSEClient 模拟旧版 SSE 传输：GET 事件流下发 endpoint，POST 消息经事件流按 id 回响应（SSE 帧格式）
func TestSSEClient(t *testing.T) {
	var mu sync.Mutex
	pending := make(map[int64]string) // id -> 待投递的 result JSON
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		fmt.Fprint(w, "event: endpoint\ndata: /msg\n\n")
		fl.Flush()
		idCh := make(chan int64, 4)
		mu.Lock()
		sseWaiters = append(sseWaiters, idCh)
		mu.Unlock()
		// 串行写流：收到 POST 的请求 id 后，把预置结果以 SSE 帧写回（仅本 handler 写 w，避免竞态）
		for {
			select {
			case id := <-idCh:
				mu.Lock()
				result := pending[id]
				delete(pending, id)
				mu.Unlock()
				payload := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":%s}`, id, result)
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", payload)
				fl.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	mux.HandleFunc("/msg", func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result string
		switch req.Method {
		case "initialize":
			result = `{"protocolVersion":"` + protocolVersion + `","serverInfo":{"name":"sse-mock","version":"1.1"},"capabilities":{}}`
		case "tools/list":
			result = `{"tools":[{"name":"add","description":"加法","inputSchema":{"type":"object"}}]}`
		case "tools/call":
			result = `{"content":[{"type":"text","text":"3"}]}`
		default:
			result = `{}`
		}
		mu.Lock()
		pending[req.ID] = result
		mu.Unlock()
		mu.Lock()
		for _, ch := range sseWaiters {
			select {
			case ch <- req.ID:
			default:
			}
		}
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cli := NewClient(ClientConfig{Transport: TransportSSE, BaseURL: srv.URL + "/sse"})
	defer cli.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, proto, err := cli.Handshake(ctx)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	if info.Name != "sse-mock" || proto != protocolVersion {
		t.Fatalf("unexpected server info: %+v %s", info, proto)
	}
	tools, err := cli.ListTools(ctx)
	if err != nil || len(tools) != 1 || tools[0].Name != "add" {
		t.Fatalf("list tools: %+v %v", tools, err)
	}
	out, err := cli.CallTool(ctx, "add", `{"a":1,"b":2}`)
	if err != nil || out != "3" {
		t.Fatalf("call tool: %q %v", out, err)
	}
}

var sseWaiters []chan int64 // 测试辅助：POST -> SSE 流分发
