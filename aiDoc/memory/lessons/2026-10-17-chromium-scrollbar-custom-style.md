<!-- last-updated: 2026-10-17 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# Chromium 自定义滚动条的两个绘制陷阱

## 情境

给管理端深色侧栏（`web/src/layout/AdminLayout.vue` 的 `.sider-menu`）定制 `::-webkit-scrollbar` 滚动条（2026-10-17，Electron 41 / Chrome 146，macOS）。

## 坑 / 模式

1. **`overflow-y: auto` 下自定义滚动条不绘制**：macOS 的 `auto` 走 overlay 滚动条，`::-webkit-scrollbar` 的宽度/颜色不生效（`offsetWidth - clientWidth` 仍能测到槽位，但看不到拇指）。改 `overflow-y: scroll` 强制经典滚动条后自定义样式才生效。
2. **容器 `:hover` 改滚动条颜色不触发重绘**：`.sider-menu:hover::-webkit-scrollbar-thumb { background: ... }` 在 hover 时不重绘滚动条层，拇指要等一次滚动/样式失效才出现。可靠做法是拇指常驻（低透明度），只对拇指自身 `::-webkit-scrollbar-thumb:hover` 增亮（指针进入拇指会正常重绘）。

排查手段：`getComputedStyle` 看 `overflowY` / `scrollbarGutter`；用 `offsetWidth - clientWidth` 判断滚动条槽位；注入高对比色 `background: red !important` 确认是否绘制；截图像素采样验证颜色变化（避免肉眼漏看低对比拇指）。

## 出现次数

1（首次建档）

## 状态

pending

## 晋升去向

未达晋升条件（出现 1 次）；若再次遇到自定义滚动条不绘制/不重绘，考虑晋升到 `aiDoc/frontend/frontend-rules.md` 样式规范节。

## 晋升后复发

0

## 记录日期

2026-10-17
