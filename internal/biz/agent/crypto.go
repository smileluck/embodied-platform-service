package agent

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// Crypto API Key 加密器（AES-256-GCM）。
// 密钥派生：显式配置 agent.cryptoKey 优先；未配置时从 jwt.secret 派生，
// 不引入必填新配置（注意：更换 jwt.secret 会使已存密钥不可解密，错误表现为 ErrDecryptFailed，
// 重新保存一次供应商密钥即可恢复）。
type Crypto struct {
	key [32]byte
}

// NewCrypto 构造加密器
func NewCrypto(cryptoKey, jwtSecret string) *Crypto {
	material := cryptoKey
	if material == "" {
		material = jwtSecret
	}
	c := &Crypto{}
	c.key = sha256.Sum256([]byte("smilex-agent:" + material))
	return c
}

// Encrypt 加密（空串原样返回，表示未配置密钥）；输出 base64(nonce | 密文)
func (c *Crypto) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	gcm, err := c.gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), nil)), nil
}

// Decrypt 解密（空串原样返回）；密钥不符或密文损坏返回 ErrDecryptFailed
func (c *Crypto) Decrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	gcm, err := c.gcm()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil || len(data) < gcm.NonceSize() {
		return "", ErrDecryptFailed
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrDecryptFailed
	}
	return string(plain), nil
}

func (c *Crypto) gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// MaskKey 生成展示用掩码：保留前 3 位与末 4 位（如 sk-****abcd）；过短全遮蔽
func MaskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:3] + "****" + key[len(key)-4:]
}
