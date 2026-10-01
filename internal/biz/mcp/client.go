package mcp

// MCP 客户端协议实现（零外部依赖，风格参照 agent/runtime.go）：
// JSON-RPC 2.0 over 两种传输 ——
//   streamable_http（2025-03-26 规范）：单端点 POST，响应体为 JSON 或 SSE 流（解析首帧），
//     initialize 响应头携带 Mcp-Session-Id 时后续请求回带；
//   sse（旧版 2024-11-05）：GET 打开事件流，从 endpoint 事件取得消息 POST 端点，
//     请求响应经事件流按 JSON-RPC id 匹配路由回调用方（长连接 + 串行分发）。
// 建连 10s 快速失败（base_url 填错/网络不通时尽早报 ErrConnectFailed，不拖成整体超时）；
// 整体超时由调用方 ctx 控制。

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	protocolVersion = "2025-03-26"
	clientName      = "smilex-admin-gin"
	clientVersion   = "1.0"
	mcpDialTimeout  = 10 * time.Second
	sseScanBuffer   = 4 << 20 // SSE 行缓冲上限（tools/list 大清单）
	bodyReadLimit   = 8 << 20 // POST 响应体读取上限
)

// mcpHTTPTransport 共享传输层（代理跟随环境变量，建连快速失败）
var mcpHTTPTransport = &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	DialContext:           (&net.Dialer{Timeout: mcpDialTimeout, KeepAlive: 30 * time.Second}).DialContext,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   mcpDialTimeout,
	ExpectContinueTimeout: time.Second,
}

// ---- JSON-RPC 2.0 wire 结构 ----

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"` // 0 = notification（无 id，无响应）
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("rpc %d: %s", e.Code, e.Message) }

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// ServerInfo initialize 返回的服务器身份
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string      `json:"protocolVersion"`
	ServerInfo      ServerInfo  `json:"serverInfo"`
	Capabilities    interface{} `json:"capabilities"`
}

// wireTool tools/list 协议 wire 结构（MCP 规范为驼峰字段；对外实体 tag 面向前端为 snake_case，经转换对齐）
type wireTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []wireTool `json:"tools"`
}

type callContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callResult struct {
	Content []callContent `json:"content"`
	IsError bool          `json:"isError"`
}

// ClientConfig 连接参数（服务器配置快照 + 解密后的 Token）
type ClientConfig struct {
	Transport Transport
	BaseURL   string
	Headers   []Header
	Token     string
}

// Client 单服务器会话客户端。streamable_http 仅持有会话 id（每次请求独立 POST，天然并发）；
// sse 持有事件流长连接（请求经 pending 表按 id 路由响应，同一连接上串行分发）。
// 并发安全。
type Client struct {
	cfg  ClientConfig
	http *http.Client

	mu        sync.Mutex
	sessionID string // streamable: Mcp-Session-Id
	nextID    int64

	// ---- sse 会话状态（mu 保护）----
	sseBody     io.ReadCloser
	sseEndpoint string
	pending     map[int64]chan *rpcResponse
	closed      bool
}

// NewClient 构建客户端（未握手，首个调用时惰性 initialize）
func NewClient(cfg ClientConfig) *Client {
	return &Client{
		cfg:     cfg,
		http:    &http.Client{Transport: mcpHTTPTransport},
		pending: make(map[int64]chan *rpcResponse),
	}
}

// Handshake initialize + notifications/initialized，返回服务器身份
func (c *Client) Handshake(ctx context.Context) (*ServerInfo, string, error) {
	var res initializeResult
	if err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": clientName, "version": clientVersion},
	}, &res); err != nil {
		return nil, "", err
	}
	// 完成通知（notification：服务端通常回 202，无响应体）
	if err := c.notify(ctx, "notifications/initialized"); err != nil {
		return nil, "", err
	}
	return &res.ServerInfo, res.ProtocolVersion, nil
}

// ListTools tools/list（Connection closed 提示重连）
func (c *Client) ListTools(ctx context.Context) ([]ToolInfo, error) {
	var res toolsListResult
	if err := c.call(ctx, "tools/list", map[string]any{}, &res); err != nil {
		return nil, err
	}
	tools := make([]ToolInfo, 0, len(res.Tools))
	for _, t := range res.Tools {
		tools = append(tools, ToolInfo{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	return tools, nil
}

// CallTool tools/call；content 文本拼接，isError=true 视为业务失败
func (c *Client) CallTool(ctx context.Context, name, args string) (string, error) {
	raw := json.RawMessage(args)
	if strings.TrimSpace(args) == "" {
		raw = json.RawMessage("{}")
	} else if !json.Valid(raw) {
		return "", fmt.Errorf("工具参数须为 JSON 对象: %s", args)
	}
	var res callResult
	if err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": raw}, &res); err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, ct := range res.Content {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if ct.Type == "text" || ct.Type == "" {
			sb.WriteString(ct.Text)
		} else {
			fmt.Fprintf(&sb, "[unsupported content type: %s]", ct.Type)
		}
	}
	out := sb.String()
	if res.IsError {
		return out, fmt.Errorf("工具执行失败: %s", out)
	}
	return out, nil
}

// Close 关闭长连接（幂等）
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
}

func (c *Client) closeLocked() {
	if c.closed {
		return
	}
	c.closed = true
	if c.sseBody != nil {
		_ = c.sseBody.Close()
		c.sseBody = nil
	}
	c.failPendingLocked(fmt.Errorf("%w: 连接已关闭", ErrConnectFailed))
}

func (c *Client) failPendingLocked(err error) {
	for id, ch := range c.pending {
		select {
		case ch <- &rpcResponse{ID: id, Error: &rpcError{Code: -1, Message: err.Error()}}:
		default:
		}
		delete(c.pending, id)
	}
}

// closedSnapshot 是否已关闭（读侧检查，重建由 Manager 负责）
func (c *Client) closedSnapshot() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// ---- 通用请求 ----

// call 发起 JSON-RPC 请求并把 result 绑定到 out（out 为 nil 时仅等待响应）
func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	if c.cfg.Transport == TransportSSE {
		return c.callSSE(ctx, method, params, out)
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()
	resp, err := c.post(ctx, rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return err
	}
	return bindResult(resp, out)
}

// notify 发送通知（无 id 无响应；streamable 服务端回 202，sse POST 回 202）
func (c *Client) notify(ctx context.Context, method string) error {
	req := rpcRequest{JSONRPC: "2.0", Method: method}
	if c.cfg.Transport == TransportSSE {
		if err := c.ensureSSE(ctx); err != nil {
			return err
		}
		return c.postEndpoint(ctx, req)
	}
	_, err := c.post(ctx, req)
	return err
}

// bindResult 响应错误码转哨兵（保持 detail 便于日志定位）
func bindResult(resp *rpcResponse, out any) error {
	if resp.Error != nil {
		return fmt.Errorf("%w: %s", ErrProtocol, resp.Error.Message)
	}
	if out == nil || len(resp.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		return fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	return nil
}

// ---- streamable HTTP ----

// post 单端点请求；解析 JSON / SSE 两种响应体格式，记录 Mcp-Session-Id。
// notification 无匹配响应时返回 nil。
func (c *Client) post(ctx context.Context, req rpcRequest) (*rpcResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	c.applyHeaders(httpReq)
	c.mu.Lock()
	sid := c.sessionID
	c.mu.Unlock()
	if req.ID != 0 && sid != "" {
		httpReq.Header.Set("Mcp-Session-Id", sid)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, mapNetErr(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" && req.ID != 0 {
		c.mu.Lock()
		c.sessionID = sid
		c.mu.Unlock()
	}
	if resp.StatusCode == http.StatusAccepted {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil, nil // notification 被接受
	}
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		return nil, fmt.Errorf("%w: HTTP %d %s", ErrConnectFailed, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if req.ID == 0 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil, nil
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return readRPCFromSSE(resp.Body, req.ID)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bodyReadLimit))
	if err != nil {
		return nil, mapNetErr(err)
	}
	var rpc rpcResponse
	if err := json.Unmarshal(data, &rpc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProtocol, err)
	}
	return &rpc, nil
}

// applyHeaders 公共头 + 自定义头 + Bearer 凭证（自定义头不得覆盖协议头与凭证）
func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for _, h := range c.cfg.Headers {
		if h.Key == "" || isProtectedHeader(h.Key) {
			continue
		}
		req.Header.Set(h.Key, h.Value)
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
}

func isProtectedHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "authorization", "content-type", "accept", "content-length", "host", "mcp-session-id", "cookie":
		return true
	}
	return false
}

// readRPCFromSSE 从 SSE 流中读取指定 id 的 JSON-RPC 响应（其余事件忽略；服务端发起的请求跳过）
func readRPCFromSSE(body io.Reader, wantID int64) (*rpcResponse, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), sseScanBuffer)
	var dataLines []string
	flush := func() *rpcResponse {
		defer func() { dataLines = nil }()
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		var rpc rpcResponse
		if err := json.Unmarshal([]byte(data), &rpc); err != nil || rpc.ID == 0 {
			return nil // 非 JSON-RPC 响应帧（注释/心跳/服务端通知）
		}
		if rpc.ID == wantID {
			return &rpc
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if r := flush(); r != nil {
				return r, nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, mapNetErr(err)
	}
	if r := flush(); r != nil {
		return r, nil
	}
	return nil, fmt.Errorf("%w: SSE 流结束前未收到 id=%d 的响应", ErrProtocol, wantID)
}

// ---- SSE（旧版传输）----

// ensureSSE 建立事件流并等待 endpoint 事件（已建立则直接返回）
func (c *Client) ensureSSE(ctx context.Context) error {
	c.mu.Lock()
	if c.sseEndpoint != "" && !c.closed {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	req.Header.Set("Accept", "text/event-stream")
	for _, h := range c.cfg.Headers {
		if h.Key != "" && !isProtectedHeader(h.Key) {
			req.Header.Set(h.Key, h.Value)
		}
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return mapNetErr(err)
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2<<10))
		_ = resp.Body.Close()
		return fmt.Errorf("%w: HTTP %d", ErrConnectFailed, resp.StatusCode)
	}

	c.mu.Lock()
	// 竞态兜底：并发 ensureSSE 只保留首个连接
	if c.sseEndpoint != "" && !c.closed {
		c.mu.Unlock()
		_ = resp.Body.Close()
		return nil
	}
	c.sseBody = resp.Body
	c.closed = false
	endpointCh := make(chan string, 1)
	go c.readLoop(endpointCh)
	c.mu.Unlock()

	select {
	case ep := <-endpointCh:
		resolved, err := resolveEndpoint(c.cfg.BaseURL, ep)
		if err != nil {
			c.Close()
			return err
		}
		c.mu.Lock()
		c.sseEndpoint = resolved
		c.mu.Unlock()
		return nil
	case <-ctx.Done():
		c.Close()
		return ErrTimeout
	case <-time.After(mcpDialTimeout):
		c.Close()
		return fmt.Errorf("%w: 未收到 endpoint 事件", ErrProtocol)
	}
}

// readLoop SSE 事件流读取循环：首个 endpoint 事件通知 endpointCh，
// 后续 JSON-RPC 响应按 id 路由到 pending；流断开时使全部等待方失败并标记关闭
func (c *Client) readLoop(endpointCh chan<- string) {
	scanner := bufio.NewScanner(c.readBody())
	scanner.Buffer(make([]byte, 0, 64<<10), sseScanBuffer)
	var eventName string
	var dataLines []string
	dispatch := func() {
		defer func() { eventName, dataLines = "", nil }()
		if len(dataLines) == 0 {
			return
		}
		data := strings.Join(dataLines, "\n")
		if eventName == "endpoint" {
			select {
			case endpointCh <- data:
			default:
			}
			return
		}
		var rpc rpcResponse
		if err := json.Unmarshal([]byte(data), &rpc); err != nil || rpc.ID == 0 {
			return // 心跳/服务端发起的请求（不支持，忽略）
		}
		c.mu.Lock()
		if ch, ok := c.pending[rpc.ID]; ok {
			delete(c.pending, rpc.ID)
			ch <- &rpc
		}
		c.mu.Unlock()
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			dispatch()
			continue
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	// 流结束（服务端关闭/网络断开）：唤醒等待方，等下次调用重建
	c.mu.Lock()
	c.sseEndpoint = ""
	if c.sseBody != nil {
		_ = c.sseBody.Close()
		c.sseBody = nil
	}
	c.failPendingLocked(fmt.Errorf("%w: 事件流已断开", ErrConnectFailed))
	c.mu.Unlock()
}

func (c *Client) readBody() io.Reader {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sseBody
}

// callSSE sse 传输请求：注册 pending → POST 消息端点 → 等待事件流回响应
func (c *Client) callSSE(ctx context.Context, method string, params any, out any) error {
	if err := c.ensureSSE(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	if c.closed || c.sseEndpoint == "" {
		c.mu.Unlock()
		return fmt.Errorf("%w: 事件流不可用", ErrConnectFailed)
	}
	c.nextID++
	id := c.nextID
	ch := make(chan *rpcResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.postEndpoint(ctx, rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return err
	}
	select {
	case resp := <-ch:
		return bindResult(resp, out)
	case <-ctx.Done():
		return ErrTimeout
	}
}

// postEndpoint 向消息端点 POST（服务端回 202，响应走事件流）
func (c *Client) postEndpoint(ctx context.Context, req rpcRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	c.mu.Lock()
	endpoint := c.sseEndpoint
	c.mu.Unlock()
	if endpoint == "" {
		return fmt.Errorf("%w: 消息端点未就绪", ErrConnectFailed)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	c.applyHeaders(httpReq)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return mapNetErr(err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d", ErrConnectFailed, resp.StatusCode)
	}
	return nil
}

// resolveEndpoint 解析 endpoint 事件给出的消息端点（可为绝对或相对路径，相对时基于事件流 URL）
func resolveEndpoint(baseURL, endpoint string) (string, error) {
	ep := strings.TrimSpace(endpoint)
	if ep == "" {
		return "", fmt.Errorf("%w: endpoint 为空", ErrProtocol)
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	ref, err := url.Parse(ep)
	if err != nil {
		return "", fmt.Errorf("%w: endpoint 非法: %v", ErrProtocol, err)
	}
	return base.ResolveReference(ref).String(), nil
}

// mapNetErr 网络错误归类：超时 → ErrTimeout，其余 → ErrConnectFailed（携带原因）
func mapNetErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return ErrTimeout
	}
	return fmt.Errorf("%w: %v", ErrConnectFailed, err)
}
