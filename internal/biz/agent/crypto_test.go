package agent

import "testing"

func TestCryptoRoundtrip(t *testing.T) {
	c := NewCrypto("test-crypto-key", "jwt-secret")
	for _, plain := range []string{"sk-abc123456789", "short", "", "中文密钥-测试内容"} {
		enc, err := c.Encrypt(plain)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		got, err := c.Decrypt(enc)
		if err != nil {
			t.Fatalf("decrypt %q: %v", plain, err)
		}
		if got != plain {
			t.Fatalf("roundtrip %q -> %q", plain, got)
		}
	}
}

func TestCryptoEmptyStaysEmpty(t *testing.T) {
	c := NewCrypto("", "jwt-secret")
	enc, err := c.Encrypt("")
	if err != nil || enc != "" {
		t.Fatalf("empty plain should stay empty, got %q err=%v", enc, err)
	}
	got, err := c.Decrypt("")
	if err != nil || got != "" {
		t.Fatalf("empty enc should stay empty, got %q err=%v", got, err)
	}
}

func TestCryptoWrongKeyFails(t *testing.T) {
	enc, err := NewCrypto("key-a", "").Encrypt("sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCrypto("key-b", "").Decrypt(enc); err != ErrDecryptFailed {
		t.Fatalf("expected ErrDecryptFailed, got %v", err)
	}
}

func TestCryptoDerivedFromJWTSecret(t *testing.T) {
	// 未配置 cryptoKey 时从 jwt.secret 派生：同 secret 可解，不同 secret 报解密失败
	enc, err := NewCrypto("", "jwt-secret-1").Encrypt("sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCrypto("", "jwt-secret-1").Decrypt(enc); err != nil {
		t.Fatalf("same jwt secret should decrypt: %v", err)
	}
	if _, err := NewCrypto("", "jwt-secret-2").Decrypt(enc); err != ErrDecryptFailed {
		t.Fatalf("different jwt secret should fail, got %v", err)
	}
}

func TestMaskKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"abc", "***"},
		{"12345678", "********"},
		{"sk-1234567890abcd", "sk-****abcd"},
	}
	for _, c := range cases {
		if got := MaskKey(c.in); got != c.want {
			t.Errorf("MaskKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
