<!-- last-updated: 2026-10-08 -->
<!-- lesson-meta: status=promoted count=2 post=0 target=aiDoc/frontend/frontend-rules.md -->
# n-select 搜索条件初值用 '' 导致 placeholder 不显示

## 情境

第 1 次：修复「消息通知」页（`web/src/views/system/Notices.vue`）搜索栏「级别 / 状态」两个 n-select 不显示 placeholder。第 2 次：「MCP 服务」页（`web/src/views/mcp/Servers.vue`）搜索栏「传输方式」n-select 同样不显示 placeholder。

## 坑 / 模式

搜索区 n-select 绑定的 query 字段若初始化为 `''`（空字符串），Naive UI 会把它当作一个「已选中的值」，因无匹配 option 渲染为空白，placeholder 不显示。正确约定：select 型筛选字段一律初始化 `null`（如 `status: null as number | null`），`null = 不筛`，清空（clearable）也归 null。容易再犯的原因：`'' || undefined` 在拼请求参数时看似等价，只有 UI 表现暴露差异。

## 出现次数

2（2026-10-08 Notices.vue；2026-10-08 Servers.vue。其余页面已是 null 写法）

## 状态

promoted

## 晋升去向

规则写入 `aiDoc/frontend/frontend-rules.md`「组件规范」：搜索区 n-select 的 query 字段一律初始化 `null`，禁止 `''`。

## 晋升后复发

0

## 记录日期

2026-10-08
