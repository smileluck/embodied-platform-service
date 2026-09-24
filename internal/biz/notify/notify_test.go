package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/smilex/smilex-admin-gin/internal/biz/monitor"
	"github.com/smilex/smilex-admin-gin/pkg/security"
)

// TestSendWebhookSignatureAndPayload 投递带签名头的 JSON，按约定验签（ts + "." + 原始请求体）
func TestSendWebhookSignatureAndPayload(t *testing.T) {
	secret := "wh-secret"
	enc, err := security.NewAESGCM("smilex-notify:", "m").Encrypt(secret)
	if err != nil {
		t.Fatal(err)
	}
	ch := &Channel{Type: ChannelWebhook, WebhookEnc: enc}

	var raw []byte
	var gotSign, gotTS string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		gotTS = r.Header.Get("X-Timestamp")
		gotSign = r.Header.Get("X-Sign")
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	ch.WebhookURL = srv.URL

	uc := &Usecase{crypto: security.NewAESGCM("smilex-notify:", "m")}
	if err := uc.sendWebhook(context.Background(), ch, SourceJobFailed, "任务失败", "内容", "规则A"); err != nil {
		t.Fatalf("sendWebhook: %v", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(gotTS + "." + string(raw)))
	if want := hex.EncodeToString(mac.Sum(nil)); gotSign != want {
		t.Fatalf("signature mismatch:\n got %s\nwant %s", gotSign, want)
	}

	var payload webhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("bad payload: %v", err)
	}
	if payload.Source != string(SourceJobFailed) || payload.RuleName != "规则A" || payload.Title != "任务失败" || payload.Content != "内容" {
		t.Fatalf("payload wrong: %+v", payload)
	}
	skew := time.Now().Unix() - payload.Timestamp
	if skew < -5 || skew > 5 {
		t.Fatalf("timestamp skew too large: %d", skew)
	}
}

// TestSendWebhookRetryOn5xx 5xx 触发一次重试后成功
func TestSendWebhookRetryOn5xx(t *testing.T) {
	enc, _ := security.NewAESGCM("smilex-notify:", "m").Encrypt("s")
	ch := &Channel{Type: ChannelWebhook, WebhookEnc: enc}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	ch.WebhookURL = srv.URL

	uc := &Usecase{crypto: security.NewAESGCM("smilex-notify:", "m")}
	if err := uc.sendWebhook(context.Background(), ch, SourceTest, "t", "c", "r"); err != nil {
		t.Fatalf("sendWebhook: %v", err)
	}
	if calls != 2 {
		t.Fatalf("want 2 calls (1 retry), got %d", calls)
	}
}

// TestSendWebhookFailure 持续 4xx 重试一次后报错（含响应摘要）
func TestSendWebhookFailure(t *testing.T) {
	enc, _ := security.NewAESGCM("smilex-notify:", "m").Encrypt("s")
	ch := &Channel{Type: ChannelWebhook, WebhookEnc: enc}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer srv.Close()
	ch.WebhookURL = srv.URL

	uc := &Usecase{crypto: security.NewAESGCM("smilex-notify:", "m")}
	err := uc.sendWebhook(context.Background(), ch, SourceTest, "t", "c", "r")
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected 403 error with body snippet, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("want 2 calls, got %d", calls)
	}
}

// TestAcquireCooldown 冷却窗口内同指纹只放行一次；不同指纹/不同规则互不影响
func TestAcquireCooldown(t *testing.T) {
	mr := miniredis.RunT(t)
	uc := &Usecase{
		crypto: security.NewAESGCM("smilex-notify:", "m"),
		rdb:    redis.NewClient(&redis.Options{Addr: mr.Addr()}),
	}
	r := &Rule{ID: 7, CooldownSeconds: 60}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if got := uc.acquireCooldown(ctx, r, "job:1"); got != (i == 0) {
			t.Fatalf("call %d: want %v", i, i == 0)
		}
	}
	if !uc.acquireCooldown(ctx, r, "job:2") {
		t.Fatal("different fingerprint should pass")
	}
	if !uc.acquireCooldown(ctx, &Rule{ID: 8, CooldownSeconds: 60}, "job:1") {
		t.Fatal("different rule should pass")
	}
}

// TestBuildMail 邮件组包：标题 Q 编码 + 正文 base64
func TestBuildMail(t *testing.T) {
	msg := string(buildMail("ops@example.com", []string{"a@x.com", "b@x.com"}, "告警：CPU 越限", "当前 95%"))
	for _, want := range []string{
		"From: ops@example.com\r\n", "To: a@x.com, b@x.com\r\n",
		"MIME-Version: 1.0\r\n", "charset=utf-8\r\n", "Content-Transfer-Encoding: base64\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("mail missing %q", want)
		}
	}
	if !strings.Contains(msg, "Subject: =?utf-8") {
		t.Fatal("subject not Q-encoded")
	}
	if len(msg) > 0 && !strings.HasSuffix(msg, "\r\n") {
		t.Fatal("mail should end with CRLF")
	}
}

// TestMetricValue 指标取值映射
func TestMetricValue(t *testing.T) {
	s := &monitor.Snapshot{CPUPercent: 10, MemPercent: 20, SwapPercent: 30}
	if MetricValue(s, MetricCPU) != 10 || MetricValue(s, MetricMem) != 20 || MetricValue(s, MetricSwap) != 30 {
		t.Fatal("metric mapping wrong")
	}
	if MetricValue(s, Metric("unknown")) != 0 {
		t.Fatal("unknown metric should be 0")
	}
}
