package platformsdk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestSign_CanonicalContract 签名串与派生密钥口径（对照平台 VerifySign 权威定义逐段核对）
func TestSign_CanonicalContract(t *testing.T) {
	s := &Signer{AppKey: "mk_test", AppSecret: "secret",
		Now:   func() time.Time { return time.Unix(1760000000, 0) },
		Nonce: func() (string, error) { return "nonce12345", nil }}

	bodySum := sha256.Sum256([]byte(`{"a":1}`))
	bodyHash := hex.EncodeToString(bodySum[:])
	// 手工拼装期望签名
	payload := CanonicalString("POST", "/open-api/v1/devices", "mk_test", "1760000000", "nonce12345", bodyHash)
	keySum := sha256.Sum256([]byte("mk_test:secret"))
	mac := hmac.New(sha256.New, []byte(hex.EncodeToString(keySum[:])))
	mac.Write([]byte(payload))
	want := hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest(http.MethodPost, "http://x/open-api/v1/devices", strings.NewReader(`{"a":1}`))
	if err := s.Sign(req, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get(HeaderSign); got != want {
		t.Errorf("签名不一致:\n got  %s\n want %s", got, want)
	}
	for _, h := range []string{HeaderAppKey, HeaderTimestamp, HeaderNonce, HeaderSign} {
		if req.Header.Get(h) == "" {
			t.Errorf("缺少签名头 %s", h)
		}
	}
}

// TestSign_EmptyBody GET/无 body 对空串取哈希
func TestSign_EmptyBody(t *testing.T) {
	s := &Signer{AppKey: "k", AppSecret: "v",
		Now: func() time.Time { return time.Unix(1, 0) }, Nonce: func() (string, error) { return "12345678", nil }}
	req, _ := http.NewRequest(http.MethodGet, "http://x/open-api/v1/ping", nil)
	if err := s.Sign(req, nil); err != nil {
		t.Fatal(err)
	}
	emptySum := sha256.Sum256([]byte{})
	payload := CanonicalString("GET", "/open-api/v1/ping", "k", "1", "12345678", hex.EncodeToString(emptySum[:]))
	keySum := sha256.Sum256([]byte("k:v"))
	mac := hmac.New(sha256.New, []byte(hex.EncodeToString(keySum[:])))
	mac.Write([]byte(payload))
	if got := req.Header.Get(HeaderSign); got != hex.EncodeToString(mac.Sum(nil)) {
		t.Errorf("空 body 应按空串哈希签名, got %s", got)
	}
}
