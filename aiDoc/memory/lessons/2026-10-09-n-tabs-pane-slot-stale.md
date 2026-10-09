<!-- last-updated: 2026-10-09 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# n-tabs 多 pane 同构内容必须加 key，否则切 tab 出现槽内容错绑

## 情境

租户门户/管理端日志页（`views/tenant-portal/Logs.vue`、`views/log/TenantLogs.vue`）：`n-tabs` 两个 `n-tab-pane` 内都是 `SearchCard + n-card(n-data-table)` 同构布局。浏览器冒烟发现：切到「操作日志」tab 后，筛选栏仍是登录 tab 的字段（用户名/IP/登录结果），而表格已是操作日志——DOM 确认非截图错觉，刷新后可稳定复现。

## 坑 / 模式

Naive UI tabs 切换 pane 时，Vue 对同位置、同组件类型的子树原地 patch 复用实例：组件 props（如 n-data-table 的 columns/data）被正确更新，但 `SearchCard` 的 slot 内容保留了上一个 pane 的旧渲染——props 更新、插槽陈旧，同一 pane 内「上半截是 A、下半截是 B」。表格数据正确极具迷惑性，纯类型检查/构建无法发现，必须真实浏览器点一次 tab 切换。两个 pane 布局越同构越容易触发。

修复：每个 pane 的 `SearchCard` 与 `n-card` 加互异的 `:key`（如 `'login-search'`/`'op-search'`），强制销毁重建。

## 出现次数

1（2026-10-09，两个同构页面同时中招）

## 状态

pending

## 晋升去向

（再出现一次或用户确认后，晋升至 frontend-rules.md 组件规范：n-tabs 多 pane 内容同构时必须给 pane 内根组件加互异 key；含 tabs 的页面验收必须真实切换每个 tab 检查）

## 晋升后复发

0

## 记录日期

2026-10-09
