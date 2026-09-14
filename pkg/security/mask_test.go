package security

import "testing"

func TestMask(t *testing.T) {
	cases := []struct {
		name  string
		rule  string
		value string
		want  string
	}{
		{"phone", "phone", "13812341234", "138****1234"},
		{"phone short", "phone", "12345", "12345"},
		{"phone empty", "phone", "", ""},
		{"email", "email", "admin@example.com", "a***@example.com"},
		{"email single char local", "email", "a@b.co", "a***@b.co"},
		{"email invalid", "email", "not-an-email", "not-an-email"},
		{"email empty domain", "email", "a@", "a@"},
		{"idcard", "idcard", "110101199003074321", "****4321"},
		{"idcard short", "idcard", "1234", "1234"},
		{"bankcard", "bankcard", "6222020200112233445", "****3445"},
		{"name", "name", "张三", "张*"},
		{"name long", "name", "欧阳娜娜", "欧***"},
		{"name single", "name", "张", "张"},
		{"ip", "ip", "192.168.1.100", "192.168.*.*"},
		{"ip ipv6 unchanged", "ip", "::1", "::1"},
		{"none", "none", "13812341234", "13812341234"},
		{"empty rule", "", "13812341234", "13812341234"},
		{"unknown rule", "whatever", "13812341234", "13812341234"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Mask(tc.rule, tc.value); got != tc.want {
				t.Fatalf("Mask(%q, %q) = %q, want %q", tc.rule, tc.value, got, tc.want)
			}
		})
	}
}
