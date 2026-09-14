// Package security 通用安全工具：XSS 清洗、SQL 注入特征检测、LIKE 通配符转义。
// 这是纵深防御的一层 —— 主防线仍是参数化查询（GORM 占位符）与前端输出转义（Vue 默认行为）。
package security

import (
	"regexp"
	"strings"
)

var (
	// htmlTagPattern HTML 标签（含自闭合），用于剥离注入的脚本/事件载体
	htmlTagPattern = regexp.MustCompile(`<[^>]*>`)
	// jsProtocolPattern javascript: 伪协议（大小写与空白混淆变体）
	jsProtocolPattern = regexp.MustCompile(`(?i)j\s*a\s*v\s*a\s*s\s*c\s*r\s*i\s*p\s*t\s*:`)
)

// sqlInjectionPatterns 高危 SQL 注入特征（刻意收窄组合型 pattern，避免正常业务文本误报）
var sqlInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)union[\s+]+(all[\s+]+)?select`),
	regexp.MustCompile(`(?i)information_schema`),
	regexp.MustCompile(`(?i)\bsleep\s*\(`),
	regexp.MustCompile(`(?i)\bbenchmark\s*\(`),
	regexp.MustCompile(`(?i)\bload_file\s*\(`),
	regexp.MustCompile(`(?i)\bxp_cmdshell\b`),
	regexp.MustCompile(`(?i)into[\s+]+(outfile|dumpfile)`),
	regexp.MustCompile(`(?i)[^\w]\b(or|and)[\s+]+\d+[\s=]+[\s]*\d+`),             // or 1=1 恒真
	regexp.MustCompile(`(?i)'\s*(or|and)\s*'`),                                   // ' or ' 恒真
	regexp.MustCompile(`(?i);[\s]*(drop|delete|update|insert|truncate|alter)\b`), // 堆叠注入
	regexp.MustCompile(`(?i)'\s*(--|#)`),                                         // 引号后接注释符截断
}

// ContainsSQLInjection 检测字符串是否含高危 SQL 注入特征
func ContainsSQLInjection(s string) bool {
	if s == "" {
		return false
	}
	for _, p := range sqlInjectionPatterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

// StripHTML 剥离 HTML 标签与 javascript: 伪协议（防御存储型 XSS；普通文本不受影响）
func StripHTML(s string) string {
	if !strings.Contains(s, "<") && !strings.ContainsAny(s, ":") {
		return s
	}
	out := htmlTagPattern.ReplaceAllString(s, "")
	out = jsProtocolPattern.ReplaceAllString(out, "")
	return out
}

const likeEscape = "/"

var likeReplacer = strings.NewReplacer(
	likeEscape, likeEscape+likeEscape,
	"%", likeEscape+"%",
	"_", likeEscape+"_",
)

// EscapeLike 转义 LIKE 通配符（%/__/转义符本身），配合 SQL 子句 ESCAPE '/' 使用，
// 防止用户输入改变匹配语义（通配符注入）；MySQL/Postgres/SQLite 通用。
func EscapeLike(s string) string {
	return likeReplacer.Replace(s)
}
