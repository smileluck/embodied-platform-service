package security

import "testing"

func TestAESGCMRoundtrip(t *testing.T) {
	c := NewAESGCM("test-domain:", "material")
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

func TestAESGCMDomainIsolation(t *testing.T) {
	// 相同 material、不同域前缀：密钥互相隔离，A 域密文在 B 域不可解
	enc, err := NewAESGCM("domain-a:", "same-material").Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewAESGCM("domain-b:", "same-material").Decrypt(enc); err != ErrAESDecryptFailed {
		t.Fatalf("cross-domain decrypt should fail, got %v", err)
	}
	if _, err := NewAESGCM("domain-a:", "same-material").Decrypt(enc); err != nil {
		t.Fatalf("same domain should decrypt: %v", err)
	}
}

func TestAESGCMWrongMaterialFails(t *testing.T) {
	enc, err := NewAESGCM("d:", "material-a").Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewAESGCM("d:", "material-b").Decrypt(enc); err != ErrAESDecryptFailed {
		t.Fatalf("expected ErrAESDecryptFailed, got %v", err)
	}
}

func TestAESGCMGarbageFails(t *testing.T) {
	c := NewAESGCM("d:", "material")
	for _, enc := range []string{"not-base64!!!", "aGk=", ""} {
		if _, err := c.Decrypt(enc); enc != "" && err != ErrAESDecryptFailed {
			t.Fatalf("Decrypt(%q) should fail, got %v", enc, err)
		}
	}
}

func TestMaskSecret(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"abc", "***"},
		{"12345678", "********"},
		{"sk-1234567890abcd", "sk-****abcd"},
	}
	for _, c := range cases {
		if got := MaskSecret(c.in); got != c.want {
			t.Errorf("MaskSecret(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
