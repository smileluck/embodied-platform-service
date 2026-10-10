package security

import "testing"

func TestContainsXSSPayload(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// 高危载体：应拦截
		{`<script>alert(1)</script>`, true},
		{`</script>`, true},
		{`<SCRIPT src=//x.com/a.js>`, true},
		{`<img src=x onerror=alert(1)>`, true},
		{`<a onclick="steal()">link</a>`, true},
		{`<iframe src="//evil"></iframe>`, true},
		{`javascript:alert(1)`, true},
		{`J a v a s c r i p t:alert(1)`, true},
		// 正常业务文本：不应误报
		{`正常关键字搜索`, false},
		{`a<b and c>d`, false},
		{`focus = true 的说明`, false},
		{`<image.png`, false},
		{`online=1`, false},
		{``, false},
	}
	for _, c := range cases {
		if got := ContainsXSSPayload(c.in); got != c.want {
			t.Errorf("ContainsXSSPayload(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestContainsSQLInjection(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{`' OR 1=1--`, true},
		{`union select * from users`, true},
		{`zhangsan`, false},
		{``, false},
	}
	for _, c := range cases {
		if got := ContainsSQLInjection(c.in); got != c.want {
			t.Errorf("ContainsSQLInjection(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
