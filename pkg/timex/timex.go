// Package timex 统一时间展示格式
package timex

import "time"

// Layout 面向直显的时间格式（yyyy-MM-dd HH:mm:ss，24 小时制本地时区）。
// 服务端 formatTime 与 CSV 导出共用此常量，禁止在业务代码散落字面量；
// 前端展示统一由 web/src/utils/datetime.ts formatDateTime 归一到同一格式。
const Layout = "2006-01-02 15:04:05"

// Format 输出 Layout 格式；零值返回空串（对应“未发生”语义）。
func Format(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(Layout)
}
