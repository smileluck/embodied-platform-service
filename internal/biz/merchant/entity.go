// Package merchant 商户（开放 API 授权）限界上下文 —— 领域层
package merchant

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"time"
)

// Status 商户状态值对象
type Status int

const (
	StatusEnabled  Status = 1
	StatusDisabled Status = 2
)

// Merchant 商户聚合根（纯 Go，不依赖任何框架）。
// AppSecretHash 只存哈希（SHA-256(AppKey + ":" + secret) 的 hex），明文 secret 仅创建/重置时返回一次。
type Merchant struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Code          string    `json:"code"`
	AppKey        string    `json:"app_key"`
	AppSecretHash string    `json:"-"`
	ContactName   string    `json:"contact_name"`
	ContactPhone  string    `json:"contact_phone"`
	ContactEmail  string    `json:"contact_email"`
	Status        Status    `json:"status"`
	Remark        string    `json:"remark"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Enabled 是否启用
func (m *Merchant) Enabled() bool { return m.Status == StatusEnabled }

// SetSecret 设置 AppSecret（只落哈希，绑定 AppKey 防跨商户换用）
func (m *Merchant) SetSecret(secret string) {
	m.AppSecretHash = hashSecret(m.AppKey, secret)
}

// CheckSecret 校验 AppSecret 明文（常量时间比对，防时序侧信道）
func (m *Merchant) CheckSecret(secret string) bool {
	return subtle.ConstantTimeCompare([]byte(m.AppSecretHash), []byte(hashSecret(m.AppKey, secret))) == 1
}

// hashSecret 计算 secret 存储哈希：SHA-256(AppKey + ":" + secret) 的 hex
func hashSecret(appKey, secret string) string {
	sum := sha256.Sum256([]byte(appKey + ":" + secret))
	return hex.EncodeToString(sum[:])
}

// APILog 开放 API 调用日志（追加型流水：无软删，保留期清理为物理删除）
type APILog struct {
	ID         uint      `json:"id"`
	MerchantID uint      `json:"merchant_id"` // 商户（鉴权失败且商户未知时为 0）
	AppKey     string    `json:"app_key"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	IP         string    `json:"ip"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int       `json:"latency_ms"`
	Msg        string    `json:"msg"` // 失败原因摘要（成功为空）
	CreatedAt  time.Time `json:"created_at"`
}
