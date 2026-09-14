// Package i18n 轻量中英文国际化：按请求 Accept-Language 头识别语言，从内置语言包查文案。
// 默认中文；当前语言缺 key 时回退中文表，仍无则返回 key 本身（调用方据此判断未命中）。
package i18n

import (
	"context"
	"fmt"
	"strings"
)

// Locale 语言标识
type Locale string

const (
	// ZhCN 简体中文（默认）
	ZhCN Locale = "zh-CN"
	// EnUS 英文
	EnUS Locale = "en-US"
)

// Detect 按 Accept-Language 头识别语言：含 zh 判为中文，含 en 判为英文，否则默认中文
func Detect(acceptLanguageHeader string) Locale {
	h := strings.ToLower(acceptLanguageHeader)
	if strings.Contains(h, "zh") {
		return ZhCN
	}
	if strings.Contains(h, "en") {
		return EnUS
	}
	return ZhCN
}

type localeKey struct{}

// WithLocale 将语言注入 context（由 I18n 中间件在请求入口调用）
func WithLocale(ctx context.Context, l Locale) context.Context {
	return context.WithValue(ctx, localeKey{}, l)
}

// FromContext 从 context 取语言，未设置时默认中文
func FromContext(ctx context.Context) Locale {
	if l, ok := ctx.Value(localeKey{}).(Locale); ok && l != "" {
		return l
	}
	return ZhCN
}

// T 按 context 中的语言查文案：当前语言缺 key 回退中文表，仍无则返回 key 本身；
// 带 args 时按 fmt.Sprintf 渲染
func T(ctx context.Context, key string, args ...any) string {
	msg, ok := messages[FromContext(ctx)][key]
	if !ok {
		msg, ok = messages[ZhCN][key]
	}
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// messages 各语言文案表（具体条目见 messages_zh.go / messages_en.go）
var messages = map[Locale]map[string]string{
	ZhCN: messagesZh,
	EnUS: messagesEn,
}
