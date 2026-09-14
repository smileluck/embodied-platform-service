package security

import "strings"

// 导出脱敏规则：config export.mask 将导出列 key 映射到下列规则名；
// 空规则 / none / 未知规则一律返回原值（显式不脱敏，避免误伤数据）
const (
	MaskRulePhone    = "phone"
	MaskRuleEmail    = "email"
	MaskRuleIDCard   = "idcard"
	MaskRuleBankCard = "bankcard"
	MaskRuleName     = "name"
	MaskRuleIP       = "ip"
	MaskRuleNone     = "none"
)

// Mask 按规则脱敏（统一入口）；空规则/none/未知规则返回原值
func Mask(rule, value string) string {
	if value == "" {
		return value
	}
	switch rule {
	case MaskRulePhone:
		return MaskPhone(value)
	case MaskRuleEmail:
		return MaskEmail(value)
	case MaskRuleIDCard, MaskRuleBankCard:
		return MaskTail4(value)
	case MaskRuleName:
		return MaskName(value)
	case MaskRuleIP:
		return MaskIP(value)
	default:
		return value
	}
}

// MaskPhone 手机号脱敏：保留前 3 后 4（138****1234），长度不足 7 位原样返回
func MaskPhone(s string) string {
	rs := []rune(s)
	if len(rs) < 7 {
		return s
	}
	return string(rs[:3]) + "****" + string(rs[len(rs)-4:])
}

// MaskEmail 邮箱脱敏：保留本地部分首字符（a***@example.com），无 @ 原样返回
func MaskEmail(s string) string {
	local, domain, ok := strings.Cut(s, "@")
	if !ok || local == "" || domain == "" {
		return s
	}
	return string([]rune(local)[:1]) + "***@" + domain
}

// MaskTail4 身份证/银行卡脱敏：仅保留后 4 位，长度不足 4 位原样返回
func MaskTail4(s string) string {
	rs := []rune(s)
	if len(rs) <= 4 {
		return s
	}
	return "****" + string(rs[len(rs)-4:])
}

// MaskName 姓名脱敏：仅保留首字符（张*、欧阳**），空串原样返回
func MaskName(s string) string {
	rs := []rune(s)
	if len(rs) <= 1 {
		return s
	}
	return string(rs[:1]) + strings.Repeat("*", len(rs)-1)
}

// MaskIP IPv4 脱敏：保留前两段（192.168.*.*），非四段格式原样返回
func MaskIP(s string) string {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return s
	}
	return parts[0] + "." + parts[1] + ".*.*"
}
