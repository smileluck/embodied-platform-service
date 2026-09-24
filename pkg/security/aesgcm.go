package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

// ErrAESDecryptFailed 解密失败：密钥不符或密文损坏（各业务域自行包装为带上下文的哨兵错误）
var ErrAESDecryptFailed = errors.New("aes-gcm decrypt failed")

// AESGCM 通用 AES-256-GCM 字符串加密器（供应商 API Key、SMTP 密码、Webhook 密钥等静态密文存储）。
// 密钥派生带域前缀（domain），不同业务域密钥互相隔离——即使底层 material 相同，
// A 域的密文在 B 域不可解。material 建议由显式配置提供，缺省可回退 jwt.secret
// （注意：更换 jwt.secret 会使已存密钥不可解密，重新保存一次即可恢复）。
type AESGCM struct {
	key [32]byte
}

// NewAESGCM 构造加密器：key = SHA256(domain + material)
func NewAESGCM(domain, material string) *AESGCM {
	c := &AESGCM{}
	c.key = sha256.Sum256([]byte(domain + material))
	return c
}

// Encrypt 加密（空串原样返回，表示未配置）；输出 base64(nonce | 密文)
func (c *AESGCM) Encrypt(plain string) (string, error) {
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

// Decrypt 解密（空串原样返回）；密钥不符或密文损坏返回 ErrAESDecryptFailed
func (c *AESGCM) Decrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	gcm, err := c.gcm()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil || len(data) < gcm.NonceSize() {
		return "", ErrAESDecryptFailed
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrAESDecryptFailed
	}
	return string(plain), nil
}

func (c *AESGCM) gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// MaskSecret 生成展示用掩码：保留前 3 位与末 4 位（如 sk-****abcd）；过短全遮蔽
func MaskSecret(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:3] + "****" + key[len(key)-4:]
}
