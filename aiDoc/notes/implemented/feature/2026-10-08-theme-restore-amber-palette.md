<!-- last-updated: 2026-10-08 -->
# 主题恢复工业琥珀色板（替换清水蓝），保留亮暗双套结构

## 问题

用户反馈登录页配色从橙红变成蓝色。「主题收敛单一来源与暗色模式」变更把整套设计 token 从工业琥珀（`#D97706`）换成了清水蓝（`#3F75AB`），登录页、按钮、图表随之变蓝；用户希望回到原配色，且暗色模式不能出显示问题。

## 提案 / 决策

在 `web/src/styles/variables.css`（主题唯一来源）恢复旧琥珀/石墨色板，不回滚暗色模式架构：

- 亮色 `:root`：主色 `#D97706` / hover `#E8960C` / pressed `#B8620A` / soft `#FBEED7` / bright `#FFB224`；面板与深壳回石墨黑 `#17191E`（`#20242C` 层次）；底色回暖瓷白 `#F4F4F2`；补回切换蓝色时丢失的 `--sx-ok` / `--sx-ok-bright` / `--sx-warn`（sx-led 状态灯与登录页脉搏依赖，缺失时静默落入 CSS 兜底）
- 新增 `--sx-accent-rgb: 217, 119, 6`，供各页面 rgba 衬底使用，替换散落的硬编码 `rgba(63, 117, 171, …)`
- 暗色 `html.dark`：保留反转结构；`--sx-accent-soft` 改琥珀 rgba；`--sx-ok` 提亮为 `#34D399`（原 `#16A34A` 在暗底上过暗）
- 暗色显示问题修复：登录页品牌印章文字色 `--sx-ink`（暗色下反转成浅色，压在恒浅琥珀底上不可读）改恒深 `--sx-panel`，与 AdminLayout 印章/头像同款处理；聊天代码块暗色覆盖（`tokens.css`）的蓝调底色改中性石墨；ServerMonitor 图表 splitLine 亮色硬编码改中性灰 rgba（亮暗两套底都可读）

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 整体回滚「主题收敛单一来源与暗色模式」提交 | 会丢失暗色模式与 token 单一来源结构；用户只要求恢复配色，不要求取消暗色模式 |
| 只改登录页局部样式 | 蓝色来自全局 token，局部改会与全站主题脱节，违背主题唯一来源约定 |

## 验收标准

- [x] `web/` 下 `npm run lint` 0 error，`npm run build`（vue-tsc -b && vite build）通过
- [x] dev server 真实入口冒烟：登录页亮色 = 暖白表单面 + 石墨品牌面板 + 琥珀主色；暗色 = 面板/主色不变、印章深字可读、状态灯磷光绿可读
- [x] 全仓 grep 无清水蓝色值残留（`#3F75AB` / `rgba(63, 117, 171, …)` 等）

## 风险与后果

- 图表（Dashboard/Usage/ServerMonitor）曲线色改琥珀系，Dashboard 操作数曲线改用中性灰 `#6E7681` 避免与琥珀撞色
- 后续主题调整仍只改 `variables.css`；新增 rgba 衬底一律用 `--sx-accent-rgb`，禁止再硬编码色值

## 交叉链接

- 前置约束：notes/implemented/bug-fix/2026-10-08-dark-mode-sider-shell-tokens.md（恒深壳元素禁用过反转 token）
- 规范：aiDoc/frontend/frontend-rules.md 样式规范节
- 业务记忆：aiDoc/memory/business/2026-10-08-主题恢复工业琥珀配色.md
