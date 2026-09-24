package agent

import (
	"strings"

	"github.com/smilex/smilex-admin-gin/pkg/security"
)

// Crypto API Key 加密器（AES-256-GCM，实现泛化于 pkg/security.AESGCM，域前缀 smilex-agent:）。
// 密钥派生：显式配置 agent.cryptoKey 优先；未配置时从 jwt.secret 派生，
// 不引入必填新配置（注意：更换 jwt.secret 会使已存密钥不可解密，错误表现为 ErrDecryptFailed，
// 重新保存一次供应商密钥即可恢复）。
type Crypto struct {
	inner *security.AESGCM
}

// NewCrypto 构造加密器
func NewCrypto(cryptoKey, jwtSecret string) *Crypto {
	material := cryptoKey
	if material == "" {
		material = jwtSecret
	}
	return &Crypto{inner: security.NewAESGCM("smilex-agent:", material)}
}

// Encrypt 加密（空串原样返回，表示未配置密钥）
func (c *Crypto) Encrypt(plain string) (string, error) { return c.inner.Encrypt(plain) }

// Decrypt 解密（空串原样返回）；密钥不符或密文损坏返回 ErrDecryptFailed
func (c *Crypto) Decrypt(enc string) (string, error) {
	plain, err := c.inner.Decrypt(enc)
	if err != nil {
		return "", ErrDecryptFailed
	}
	return plain, nil
}

// MaskKey 生成展示用掩码（泛化实现见 security.MaskSecret）
func MaskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:3] + "****" + key[len(key)-4:]
}
