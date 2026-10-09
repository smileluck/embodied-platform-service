<!-- last-updated: 2026-10-09 -->
# 服务器监控页整页白屏——`<script setup>` 顶层 TDZ 引用未初始化 ref

## 问题

管理端「服务器监控」(`/system/monitor`) 整页空白：进入该路由后整个应用卸载，DOM 完全为空（不是仅页面无数据）。后端接口全部正常（`/api/v1/monitor`、`/monitor/history` 均 200）。根因：`ServerMonitor.vue` 主题切换改动中，setup 顶层第 158 行直接调用 `refreshAccent()`，其内部写 `ACCENT_SOFT.value`；而 `const ACCENT_SOFT = ref('#FFB224')` 声明在第 164 行（watch 注册之后）——`const` 暂时性死区（TDZ），执行即抛 `ReferenceError: Cannot access 'ACCENT_SOFT' before initialization`，setup 崩溃导致整应用白屏。与用户确认的「更早就有」（主题切换功能上线后）吻合。

## 提案 / 决策

把 `const ACCENT_SOFT = ref('#FFB224')` 声明上移到 `ACCENT` 之后、`refreshAccent` 定义与调用之前，消除 TDZ。纯声明顺序调整，无行为变化。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 在 `refreshAccent` 内改用可选链/try-catch 防御 | 掩盖 TDZ 而非修复；配色会静默回落默认值，主题切换失效 |
| 把 `refreshAccent()` 首次调用推迟到 onMounted | 多此一举；声明顺序修正后顶层调用本就合法，且首帧即可取到主题变量 |
| 全局 `app.config.errorHandler` 兜底防白屏 | 独立议题（错误边界策略），不能替代本 bug 修复 |

## 验收标准

- [x] 浏览器真实会话（ego 登录）进入 `/system/monitor`：页面完整渲染，CPU/内存/磁盘/网络/Go 运行时/历史回看全部展示，echarts 曲线正常绘制
- [x] `cd web && npm run build` 通过（5.70s，仅既有 chunk 体积警告）

## 风险与后果

- 无接口/数据变化；纯前端声明顺序修复。
- 同类隐患已入 lessons（`setup-tdz-ref-before-declaration`）：「页面白屏」排查先怀疑 setup 顶层 TDZ。

## 交叉链接

- 代码：`web/src/views/system/ServerMonitor.vue:152-164`（ACCENT/ACCENT_SOFT/refreshAccent 声明顺序）
- 经验：`aiDoc/memory/lessons/2026-10-09-setup-tdz-ref-before-declaration.md`
