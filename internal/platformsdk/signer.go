// Package platformsdk embodied-platform 开放 API（/open-api/v1）Go 客户端。
// 仅标准库依赖；禁止 import 平台 internal/（共享契约类型在本包镜像维护，
// 由平台仓库 internal/service/openapi/contract_test.go 做字段防漂移比对）。
//
// 本包是 embodied-platform/sdk 的内联副本（原独立 module 经 go.mod replace 引入，
// 为使 CI/Docker 构建无需本地路径与私有凭据改为仓库内维护）：
// 平台开放面演进后需将 ../embodied-platform/sdk/*.go 的改动同步复制过来，保持两边一致。
package platformsdk

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// 签名请求头（与平台 internal/server/middleware/openapi.go 约定一致）
const (
	HeaderAppKey    = "X-App-Key"
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"
	HeaderSign      = "X-Sign"
)

// Signer 商户 HMAC 签名器。
// 算法（唯一权威定义在平台 internal/biz/merchant/usecase.go VerifySign）：
//
//	derived_key = hex(SHA-256(appKey + ":" + appSecret))   // 64 位 hex 字符串直接作 HMAC 密钥
//	body_hash   = hex(SHA-256(请求 body 原始字节))           // GET/无 body 对空串取哈希
//	payload     = METHOD \n path \n appKey \n timestamp \n nonce \n body_hash
//	X-Sign      = hex(HMAC-SHA256(derived_key, payload))
//
// path 是**实际发送的完整路径**（不含 query）：直连签 /open-api/v1/...，经 /gw 网关签 /gw/open-api/v1/...。
type Signer struct {
	AppKey    string
	AppSecret string
	// Now 时间源（测试注入）；nil 用 time.Now
	Now func() time.Time
	// Nonce 随机源（测试注入）；nil 用 crypto/rand
	Nonce func() (string, error)
}

// NewSigner 构造签名器
func NewSigner(appKey, appSecret string) *Signer {
	return &Signer{AppKey: appKey, AppSecret: appSecret}
}

// Sign 给请求打上四个签名头。body 为发送的原始字节（GET/无 body 传 nil）。
func (s *Signer) Sign(req *http.Request, body []byte) error {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	newNonce := func() (string, error) {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		return hex.EncodeToString(b), nil
	}
	if s.Nonce != nil {
		newNonce = s.Nonce
	}
	nonce, err := newNonce()
	if err != nil {
		return fmt.Errorf("sdk: generate nonce: %w", err)
	}

	ts := strconv.FormatInt(now.Unix(), 10)
	bodySum := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(bodySum[:])

	payload := req.Method + "\n" + req.URL.Path + "\n" + s.AppKey + "\n" + ts + "\n" + nonce + "\n" + bodyHash
	keySum := sha256.Sum256([]byte(s.AppKey + ":" + s.AppSecret))
	derived := hex.EncodeToString(keySum[:])
	mac := hmac.New(sha256.New, []byte(derived))
	mac.Write([]byte(payload))

	req.Header.Set(HeaderAppKey, s.AppKey)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSign, hex.EncodeToString(mac.Sum(nil)))
	return nil
}

// CanonicalString 拼装签名串（导出供联调排查：客户端可对照服务端排错文档逐段核对）
func CanonicalString(method, path, appKey, timestamp, nonce, bodyHash string) string {
	return method + "\n" + path + "\n" + appKey + "\n" + timestamp + "\n" + nonce + "\n" + bodyHash
}
