package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// fakeRepo 内存版智能体仓储（byCode/byName 查重、删除保护计数均模拟真实语义）
type fakeRepo struct {
	providers []*Provider
	models    []*Model
	agents    []*Agent
	nextID    uint
}

func newFakeRepo() *fakeRepo { return &fakeRepo{nextID: 100} }

func (r *fakeRepo) saveID() uint { r.nextID++; return r.nextID }

func (r *fakeRepo) CreateProvider(_ context.Context, p *Provider) error {
	p.ID = r.saveID()
	r.providers = append(r.providers, p)
	return nil
}
func (r *fakeRepo) UpdateProvider(_ context.Context, p *Provider) error {
	for i, x := range r.providers {
		if x.ID == p.ID {
			r.providers[i] = p
			return nil
		}
	}
	return ErrProviderNotFound
}
func (r *fakeRepo) DeleteProvider(_ context.Context, id uint) error {
	for i, x := range r.providers {
		if x.ID == id {
			r.providers = append(r.providers[:i], r.providers[i+1:]...)
			return nil
		}
	}
	return ErrProviderNotFound
}
func (r *fakeRepo) FindProviderByID(_ context.Context, id uint) (*Provider, error) {
	for _, x := range r.providers {
		if x.ID == id {
			return x, nil
		}
	}
	return nil, ErrProviderNotFound
}
func (r *fakeRepo) FindProviderByCode(_ context.Context, code string) (*Provider, error) {
	for _, x := range r.providers {
		if x.Code == code {
			return x, nil
		}
	}
	return nil, ErrProviderNotFound
}
func (r *fakeRepo) ListProviders(context.Context, ProviderQuery, int, int) ([]*Provider, int64, error) {
	return r.providers, int64(len(r.providers)), nil
}
func (r *fakeRepo) CountModelsByProvider(_ context.Context, providerID uint) (int64, error) {
	var n int64
	for _, m := range r.models {
		if m.ProviderID == providerID {
			n++
		}
	}
	return n, nil
}

func (r *fakeRepo) CreateModel(_ context.Context, m *Model) error {
	m.ID = r.saveID()
	r.models = append(r.models, m)
	return nil
}
func (r *fakeRepo) UpdateModel(_ context.Context, m *Model) error {
	for i, x := range r.models {
		if x.ID == m.ID {
			r.models[i] = m
			return nil
		}
	}
	return ErrModelNotFound
}
func (r *fakeRepo) DeleteModel(_ context.Context, id uint) error {
	for i, x := range r.models {
		if x.ID == id {
			r.models = append(r.models[:i], r.models[i+1:]...)
			return nil
		}
	}
	return ErrModelNotFound
}
func (r *fakeRepo) FindModelByID(_ context.Context, id uint) (*Model, error) {
	for _, x := range r.models {
		if x.ID == id {
			return x, nil
		}
	}
	return nil, ErrModelNotFound
}
func (r *fakeRepo) FindModelByName(_ context.Context, providerID uint, name string) (*Model, error) {
	for _, x := range r.models {
		if x.ProviderID == providerID && x.Name == name {
			return x, nil
		}
	}
	return nil, ErrModelNotFound
}
func (r *fakeRepo) FindFirstEnabledModel(_ context.Context, providerID uint) (*Model, error) {
	for _, x := range r.models {
		if x.ProviderID == providerID && x.Status == StatusEnabled {
			return x, nil
		}
	}
	return nil, ErrProviderNoModels
}
func (r *fakeRepo) ListModels(context.Context, ModelQuery, int, int) ([]*Model, int64, error) {
	return r.models, int64(len(r.models)), nil
}
func (r *fakeRepo) CountAgentsByModel(_ context.Context, modelID uint) (int64, error) {
	var n int64
	for _, a := range r.agents {
		if a.ModelID == modelID {
			n++
		}
	}
	return n, nil
}

func (r *fakeRepo) CreateAgent(_ context.Context, a *Agent) error {
	a.ID = r.saveID()
	r.agents = append(r.agents, a)
	return nil
}
func (r *fakeRepo) UpdateAgent(_ context.Context, a *Agent) error {
	for i, x := range r.agents {
		if x.ID == a.ID {
			r.agents[i] = a
			return nil
		}
	}
	return ErrAgentNotFound
}
func (r *fakeRepo) DeleteAgent(_ context.Context, id uint) error {
	for i, x := range r.agents {
		if x.ID == id {
			r.agents = append(r.agents[:i], r.agents[i+1:]...)
			return nil
		}
	}
	return ErrAgentNotFound
}
func (r *fakeRepo) FindAgentByID(_ context.Context, id uint) (*Agent, error) {
	for _, x := range r.agents {
		if x.ID == id {
			return x, nil
		}
	}
	return nil, ErrAgentNotFound
}
func (r *fakeRepo) FindAgentByCode(_ context.Context, code string) (*Agent, error) {
	for _, x := range r.agents {
		if x.Code == code {
			return x, nil
		}
	}
	return nil, ErrAgentNotFound
}
func (r *fakeRepo) ListAgents(context.Context, AgentQuery, int, int) ([]*Agent, int64, error) {
	return r.agents, int64(len(r.agents)), nil
}

func newTestUsecase(repo Repo) *Usecase {
	return NewUsecase(repo, &conf.Bootstrap{
		Agent: conf.Agent{CryptoKey: "unit-test-key"},
		JWT:   conf.JWT{Secret: "jwt-secret"},
	}, nil)
}

// seed 三层配置：供应商（带密钥）-> 模型 -> Agent
func seed(t *testing.T, uc *Usecase) (*Provider, *Model, *Agent) {
	t.Helper()
	p, err := uc.CreateProvider(context.Background(), ProviderInput{
		Name: "智谱", Code: "zhipu", BaseURL: "http://example.invalid/v4",
		APIKey: "sk-1234567890abcd", Status: StatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := uc.CreateModel(context.Background(), ModelInput{
		ProviderID: p.ID, Name: "glm-4.7", DisplayName: "GLM 4.7", Status: StatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	a, err := uc.CreateAgent(context.Background(), AgentInput{
		Name: "助手", Code: "assistant", ModelID: m.ID, SystemPrompt: "你是测试助手",
		Status: StatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p, m, a
}

func TestProviderKeyEncryptedAtRest(t *testing.T) {
	repo := newFakeRepo()
	uc := newTestUsecase(repo)
	p, _, _ := seed(t, uc)

	// 密文入库（非明文）、掩码可展示、可回环解密
	if p.APIKeyEnc == "" || strings.Contains(p.APIKeyEnc, "sk-1234567890abcd") {
		t.Fatalf("api key must be encrypted, got %q", p.APIKeyEnc)
	}
	if p.APIKeyMask != "sk-****abcd" {
		t.Fatalf("mask = %q", p.APIKeyMask)
	}
	stored, _ := repo.FindProviderByID(context.Background(), p.ID)
	plain, err := uc.crypto.Decrypt(stored.APIKeyEnc)
	if err != nil || plain != "sk-1234567890abcd" {
		t.Fatalf("decrypt roundtrip failed: %q %v", plain, err)
	}

	// 更新时 API Key 留空 = 保持原密钥
	if err := uc.UpdateProvider(context.Background(), p.ID, ProviderInput{
		Name: "智谱2", Code: "zhipu", BaseURL: "http://example.invalid/v4", Status: StatusEnabled,
	}); err != nil {
		t.Fatal(err)
	}
	after, _ := repo.FindProviderByID(context.Background(), p.ID)
	if after.APIKeyEnc != stored.APIKeyEnc || after.Name != "智谱2" {
		t.Fatal("empty api key update must keep original key")
	}
}

func TestDuplicateCodeRejected(t *testing.T) {
	uc := newTestUsecase(newFakeRepo())
	_, m, _ := seed(t, uc)
	if _, err := uc.CreateProvider(context.Background(), ProviderInput{Name: "x", Code: "zhipu", BaseURL: "http://a", Status: StatusEnabled}); err != ErrProviderCodeExists {
		t.Fatalf("provider duplicate code: %v", err)
	}
	if _, err := uc.CreateAgent(context.Background(), AgentInput{Name: "x", Code: "assistant", ModelID: m.ID, Status: StatusEnabled}); err != ErrAgentCodeExists {
		t.Fatalf("agent duplicate code: %v", err)
	}
}

func TestDeleteProtection(t *testing.T) {
	uc := newTestUsecase(newFakeRepo())
	p, m, _ := seed(t, uc)

	if err := uc.DeleteProvider(context.Background(), p.ID); err != ErrProviderHasModels {
		t.Fatalf("provider with models must be rejected, got %v", err)
	}
	if err := uc.DeleteModel(context.Background(), m.ID); err != ErrModelInUse {
		t.Fatalf("model referenced by agent must be rejected, got %v", err)
	}
}

func TestModelDuplicateInProvider(t *testing.T) {
	uc := newTestUsecase(newFakeRepo())
	p, _, _ := seed(t, uc)
	_, err := uc.CreateModel(context.Background(), ModelInput{
		ProviderID: p.ID, Name: "glm-4.7", Status: StatusEnabled,
	})
	if err != ErrModelExists {
		t.Fatalf("expected ErrModelExists, got %v", err)
	}
}

// newSSEUpstream 模拟 OpenAI 兼容流式上游
func newSSEUpstream(t *testing.T, got *[]Message) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := decodeJSON(r, &req); err != nil {
			t.Errorf("decode: %v", err)
		}
		*got = req.Messages
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"回复\"},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func TestChatStreamEndToEnd(t *testing.T) {
	var upstreamMessages []Message
	srv := newSSEUpstream(t, &upstreamMessages)
	defer srv.Close()

	uc := newTestUsecase(newFakeRepo())
	p, m, a := seed(t, uc)
	// 指向本地模拟上游
	if err := uc.UpdateProvider(context.Background(), p.ID, ProviderInput{
		Name: "模拟", Code: "zhipu", BaseURL: srv.URL, Status: StatusEnabled,
	}); err != nil {
		t.Fatal(err)
	}

	ch, meta, err := uc.ChatStream(context.Background(), a.ID, 0, 0, []Message{{Role: "user", Content: "你好"}})
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("stream error: %v", ev.Err)
		}
		sb.WriteString(ev.Delta)
	}
	if sb.String() != "回复" {
		t.Fatalf("stream = %q", sb.String())
	}
	if meta.Model != m.Name || meta.AgentID != a.ID {
		t.Fatalf("meta = %+v", meta)
	}
	// system prompt 由配置注入，排在用户消息之前
	if len(upstreamMessages) != 2 || upstreamMessages[0].Role != "system" || upstreamMessages[0].Content != "你是测试助手" || upstreamMessages[1].Role != "user" {
		t.Fatalf("upstream messages = %+v", upstreamMessages)
	}
}

func TestChatStreamDisabledAgentRejected(t *testing.T) {
	uc := newTestUsecase(newFakeRepo())
	_, _, a := seed(t, uc)
	a.Status = StatusDisabled
	if _, _, err := uc.ChatStream(context.Background(), a.ID, 0, 0, []Message{{Role: "user", Content: "hi"}}); err != ErrAgentDisabled {
		t.Fatalf("expected ErrAgentDisabled, got %v", err)
	}
}

func TestTestProviderNoModels(t *testing.T) {
	uc := newTestUsecase(newFakeRepo())
	p, err := uc.CreateProvider(context.Background(), ProviderInput{
		Name: "空", Code: "empty", BaseURL: "http://example.invalid", Status: StatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uc.TestProvider(context.Background(), p.ID, 0); err != ErrProviderNoModels {
		t.Fatalf("expected ErrProviderNoModels, got %v", err)
	}
}

// ---- 会话接口的空实现（fakeRepo 不参与会话用例测试，仅满足接口） ----

func (r *fakeRepo) CreateConversation(_ context.Context, cv *Conversation) error { return nil }
func (r *fakeRepo) UpdateConversation(_ context.Context, cv *Conversation) error { return nil }
func (r *fakeRepo) DeleteConversation(_ context.Context, userID, id uint) error  { return nil }
func (r *fakeRepo) FindConversation(_ context.Context, userID, id uint) (*Conversation, error) {
	return nil, ErrConversationNotFound
}
func (r *fakeRepo) ListConversations(_ context.Context, userID uint, q ConversationQuery, page, pageSize int) ([]*Conversation, int64, error) {
	return nil, 0, nil
}
func (r *fakeRepo) AppendMessage(_ context.Context, m *ConversationMessage) error { return nil }
func (r *fakeRepo) ListMessages(_ context.Context, conversationID uint, page, pageSize int) ([]*ConversationMessage, int64, error) {
	return nil, 0, nil
}

// ---- 用量接口的空实现（fakeRepo 不参与用量测试，仅满足接口） ----

func (r *fakeRepo) AppendUsage(_ context.Context, u *UsageLog) error { return nil }
func (r *fakeRepo) ListUsageSince(_ context.Context, since time.Time) ([]*UsageLog, error) {
	return nil, nil
}
func (r *fakeRepo) CleanupUsageBefore(_ context.Context, before time.Time) error { return nil }
