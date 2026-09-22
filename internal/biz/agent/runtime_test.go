package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient 构建指向 httptest 上游的客户端
func newTestClient(srv *httptest.Server, apiKey string) LLMClient {
	return NewLLMClient(&Provider{BaseURL: srv.URL, Protocol: ProtocolOpenAI}, apiKey)
}

func TestChatCompletionOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("missing bearer token, got %q", got)
		}
		var body ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		fmt.Fprint(w, `{"model":"glm-4.7","choices":[{"message":{"role":"assistant","content":"你好"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`)
	}))
	defer srv.Close()

	resp, err := newTestClient(srv, "sk-test").ChatCompletion(context.Background(), ChatRequest{
		Model: "glm-4.7", Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "你好" || resp.Usage.TotalTokens != 7 {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestChatCompletionUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Incorrect API key"}}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv, "sk-bad").ChatCompletion(context.Background(), ChatRequest{Model: "m"})
	if !errors.Is(err, ErrLLMUpstream) {
		t.Fatalf("expected ErrLLMUpstream, got %v", err)
	}
	if detail := UpstreamDetail(err); !strings.Contains(detail, "Incorrect API key") {
		t.Fatalf("detail should carry upstream message, got %q", detail)
	}
}

func TestStreamCompletion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != true {
			t.Errorf("stream must be true, got %v", body["stream"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		// 两帧增量 + usage 帧 + DONE；混入注释行与非 data 行验证解析健壮性
		fmt.Fprint(w, ": keepalive\n\nevent: message\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	ch, err := newTestClient(srv, "").StreamCompletion(context.Background(), ChatRequest{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	var usage *Usage
	finish := ""
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("stream error: %v", ev.Err)
		}
		sb.WriteString(ev.Delta)
		if ev.FinishReason != "" {
			finish = ev.FinishReason
		}
		if ev.Usage != nil {
			usage = ev.Usage
		}
	}
	if sb.String() != "你好" {
		t.Fatalf("delta = %q, want 你好", sb.String())
	}
	if finish != "stop" {
		t.Fatalf("finish = %q", finish)
	}
	if usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestStreamCompletionNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"message":"no quota"}}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv, "").StreamCompletion(context.Background(), ChatRequest{Model: "m"})
	if !errors.Is(err, ErrLLMUpstream) || !strings.Contains(UpstreamDetail(err), "no quota") {
		t.Fatalf("expected upstream error with detail, got %v", err)
	}
}

func TestListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"data":[{"id":"glm-4.7"},{"id":"glm-4-flash"},{"id":"glm-4.7"}]}`)
	}))
	defer srv.Close()

	models, err := newTestClient(srv, "").ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "glm-4-flash" || models[1] != "glm-4.7" {
		t.Fatalf("models = %v", models)
	}
}

func TestChatCompletionTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := newTestClient(srv, "").ChatCompletion(ctx, ChatRequest{Model: "m"})
	if !errors.Is(err, ErrLLMTimeout) {
		t.Fatalf("expected ErrLLMTimeout, got %v", err)
	}
}

// mapTransportErr 分类：建连阶段超时应归为 ErrLLMConnect（提示检查配置），
// 不能与整体生成超时 ErrLLMTimeout 混淆。
func TestMapTransportErrDialTimeout(t *testing.T) {
	// 模拟拨号超时：net.Error 且 Timeout()=true，但不是 ctx 截止
	dialErr := &fakeNetTimeoutError{}
	err := mapTransportErr(dialErr)
	if !errors.Is(err, ErrLLMConnect) {
		t.Fatalf("expected ErrLLMConnect, got %v", err)
	}
	// ctx 截止（整体超时）仍归 ErrLLMTimeout
	if got := mapTransportErr(fmt.Errorf("wrapped: %w", context.DeadlineExceeded)); !errors.Is(got, ErrLLMTimeout) {
		t.Fatalf("expected ErrLLMTimeout, got %v", got)
	}
	// 普通传输错误归 ErrLLMUpstream 并携带原始信息
	got := mapTransportErr(errors.New("connection refused"))
	if !errors.Is(got, ErrLLMUpstream) || !strings.Contains(got.Error(), "connection refused") {
		t.Fatalf("expected ErrLLMUpstream with detail, got %v", got)
	}
}

// fakeNetTimeoutError 仅实现 Timeout()=true 的 net.Error 形态
type fakeNetTimeoutError struct{}

func (e *fakeNetTimeoutError) Error() string { return "dial tcp: i/o timeout" }
func (e *fakeNetTimeoutError) Timeout() bool { return true }
