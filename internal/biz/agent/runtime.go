package agent

// LLM 调用运行时（harness 核心）：OpenAI Chat Completions 兼容协议原生实现，
// 覆盖 OpenAI / DeepSeek / Qwen / GLM / Kimi / Ollama / vLLM 等主流服务，零外部依赖。
// 后续业务适配：经 LLMClient 接口注入自定义实现，或按 Provider.Protocol 分发新协议。

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Message 对话消息（OpenAI 兼容 role/content）
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// Usage token 用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatRequest LLM 调用参数（业务模块经此结构发起调用）
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// ChatResponse 非流式调用结果
type ChatResponse struct {
	Content string `json:"content"`
	Model   string `json:"model"`
	Usage   Usage  `json:"usage"`
}

// StreamEvent 流式事件：delta 为增量内容，结束帧携带 finish_reason 与 usage；
// Err 非空表示流中断（连接断开/上游异常），随后通道关闭
type StreamEvent struct {
	Delta        string `json:"delta,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
	Err          error  `json:"-"`
}

// LLMClient LLM 调用客户端抽象
type LLMClient interface {
	// ChatCompletion 非流式补全
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// StreamCompletion 流式补全（SSE；通道关闭即结束，出错时先发出 Err 事件再关闭）
	StreamCompletion(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
	// ListModels 拉取上游模型列表（录入辅助）
	ListModels(ctx context.Context) ([]string, error)
}

// llmHTTPTimeout 单次调用整体超时（含流式读 Body）；更长的生成由调用方 ctx 取消（前端停止按钮）
const llmHTTPTimeout = 120 * time.Second

// openaiClient OpenAI 兼容实现
type openaiClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewLLMClient 按供应商配置构建客户端
func NewLLMClient(p *Provider, apiKey string) LLMClient {
	return &openaiClient{
		baseURL: strings.TrimRight(p.BaseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: llmHTTPTimeout},
	}
}

// UpstreamDetail 提取上游错误的详细原因（渲染 agent.upstream_error 参数用）
func UpstreamDetail(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimPrefix(err.Error(), ErrLLMUpstream.Error()+": ")
}

// wire 请求/响应结构（仅运行时内部使用）

type wireStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type wireChatRequest struct {
	Model         string             `json:"model"`
	Messages      []Message          `json:"messages"`
	Temperature   float64            `json:"temperature,omitempty"`
	TopP          float64            `json:"top_p,omitempty"`
	MaxTokens     int                `json:"max_tokens,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StreamOptions *wireStreamOptions `json:"stream_options,omitempty"`
}

type wireChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

type wireStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
}

// wireAPIError 上游错误体（OpenAI 风格 error.message；部分服务直接返回 message）
type wireAPIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
	Message string `json:"message"`
}

func (c *openaiClient) newRequest(ctx context.Context, method, path string, payload []byte, accept string) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	// 本地服务（如 Ollama）可不配密钥
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return req, nil
}

// mapTransportErr 传输层错误映射：超时类归并为 ErrLLMTimeout，其余保留原始信息
func mapTransportErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w", ErrLLMTimeout)
	}
	var ne netError
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("%w", ErrLLMTimeout)
	}
	return fmt.Errorf("%w: %v", ErrLLMUpstream, err)
}

// netError net.Error 本地别名（避免导出接口实现断言歧义）
type netError = interface {
	error
	Timeout() bool
}

// upstreamErr 组装携带上游状态码与错误信息的上游错误
func upstreamErr(status int, body []byte) error {
	var we wireAPIError
	_ = json.Unmarshal(body, &we)
	msg := we.Error.Message
	if msg == "" {
		msg = we.Message
	}
	if msg == "" {
		msg = strings.TrimSpace(string(body))
		if len(msg) > 300 {
			msg = msg[:300]
		}
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return fmt.Errorf("%w: HTTP %d: %s", ErrLLMUpstream, status, msg)
}

// readBody 读取响应体（上限 4MB，防御异常上游撑爆内存）
func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

func (c *openaiClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	payload, err := json.Marshal(wireChatRequest{
		Model: req.Model, Messages: req.Messages,
		Temperature: req.Temperature, TopP: req.TopP, MaxTokens: req.MaxTokens,
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := c.newRequest(ctx, http.MethodPost, "/chat/completions", payload, "")
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, mapTransportErr(err)
	}
	data, err := readBody(resp)
	if err != nil {
		return nil, mapTransportErr(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, upstreamErr(resp.StatusCode, data)
	}
	var wr wireChatResponse
	if err := json.Unmarshal(data, &wr); err != nil {
		return nil, fmt.Errorf("%w: 响应解析失败: %v", ErrLLMUpstream, err)
	}
	if len(wr.Choices) == 0 {
		return nil, fmt.Errorf("%w: 响应缺少 choices", ErrLLMUpstream)
	}
	return &ChatResponse{Content: wr.Choices[0].Message.Content, Model: wr.Model, Usage: wr.Usage}, nil
}

func (c *openaiClient) StreamCompletion(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	payload, err := json.Marshal(wireChatRequest{
		Model: req.Model, Messages: req.Messages,
		Temperature: req.Temperature, TopP: req.TopP, MaxTokens: req.MaxTokens,
		Stream:        true,
		StreamOptions: &wireStreamOptions{IncludeUsage: true}, // 主流兼容实现忽略未知字段；支持者据此在末帧回 usage
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := c.newRequest(ctx, http.MethodPost, "/chat/completions", payload, "text/event-stream")
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, mapTransportErr(err)
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := readBody(resp)
		return nil, upstreamErr(resp.StatusCode, data)
	}

	ch := make(chan StreamEvent)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		// 单行上限 1MB（长 delta）；默认 64KB 缓冲
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data:") {
				continue // 忽略 event:/注释/空行
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}
			var chunk wireStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue // 跳过无法解析的帧，流不中断
			}
			ev := StreamEvent{Usage: chunk.Usage}
			if len(chunk.Choices) > 0 {
				ev.Delta = chunk.Choices[0].Delta.Content
				ev.FinishReason = chunk.Choices[0].FinishReason
			}
			if ev.Delta == "" && ev.FinishReason == "" && ev.Usage == nil {
				continue
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
		if err := sc.Err(); err != nil && ctx.Err() == nil {
			select {
			case ch <- StreamEvent{Err: mapTransportErr(err)}:
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

// ListModels 拉取上游 /models 列表（升序去重）
func (c *openaiClient) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := c.newRequest(ctx, http.MethodGet, "/models", nil, "")
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, mapTransportErr(err)
	}
	data, err := readBody(resp)
	if err != nil {
		return nil, mapTransportErr(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, upstreamErr(resp.StatusCode, data)
	}
	var wr struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &wr); err != nil {
		return nil, fmt.Errorf("%w: 响应解析失败: %v", ErrLLMUpstream, err)
	}
	seen := make(map[string]struct{}, len(wr.Data))
	out := make([]string, 0, len(wr.Data))
	for _, m := range wr.Data {
		if m.ID == "" {
			continue
		}
		if _, ok := seen[m.ID]; ok {
			continue
		}
		seen[m.ID] = struct{}{}
		out = append(out, m.ID)
	}
	sort.Strings(out)
	return out, nil
}

// ensure compile-time interface implementation
var _ LLMClient = (*openaiClient)(nil)
