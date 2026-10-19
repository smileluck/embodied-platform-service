<!-- last-updated: 2026-10-17 -->
# 侧边栏滚动条贴合深壳：6px 常驻细拇指 + 稳定槽位

## 问题

管理端侧栏是亮暗不变的深色壳（`--sx-shell: #17191E`），但 `.sider-menu` 仅设置 `overflow-y: auto; scrollbar-width: thin`，滚动条沿用系统配色：浅色轨道/拇指在石墨底上割出一条亮带；在 macOS 上 `auto` 还会走 overlay 滚动条，自定义样式不绘制，跨平台表现不一致。另外菜单展开/收起分组时溢出状态切换，滚动条出现/消失会使菜单内容横向跳动 4–8px。

## 提案 / 决策

在 `AdminLayout.vue` 的 `.sider-menu` 上落地深壳滚动条：

- `overflow-y: scroll`：macOS 的 `auto` 使用 overlay 滚动条，`::-webkit-scrollbar` 宽度/颜色不生效；`scroll` 强制经典滚动条，自定义样式才绘制
- `scrollbar-gutter: stable`：常驻预留滚动条槽位，展开/收起分组导致溢出状态切换时菜单内容不横向跳动
- `::-webkit-scrollbar` 6px 宽、轨道/角落透明、拇指圆角 999px；拇指常驻 `--sx-shell-scrollbar`（暖白 24%），拇指自身 `:hover` 增亮至 `--sx-shell-scrollbar-hover`（45%）
- Firefox 走 `@supports (-moz-appearance: none)` 内的 `scrollbar-width: thin` + `scrollbar-color`，避免 Chrome 121+ 也吃标准属性而覆盖 6px 宽度
- 新增 token 放在 `variables.css` 深壳 token 组（亮暗不变），遵守「恒深底元素禁用随主题反转 token」的约束

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 只保留 `scrollbar-width: thin`，用 `scrollbar-color` 改颜色 | 无法控制圆角/轨道透明；macOS `overflow-y: auto` 的 overlay 滚动条不绘制自定义拇指，跨端不一致 |
| 透明拇指 + 容器 `:hover` 显影 | 实测 Chromium 不会因容器 hover 重绘滚动条层，拇指要等一次滚动/样式失效才出现，交互不可预期 |
| 完全隐藏滚动条（`scrollbar-width: none` + `::-webkit-scrollbar { display: none }`） | 失去滚动位置感与拖拽入口，长菜单不可发现 |
| `overflow: overlay` | 非标准属性，已被 Chromium 移除，且 Firefox 不支持 |

## 验收标准

- [x] `web/` 下 `npm run lint` 0 error（仅存量 warning），`npm run build`（vue-tsc -b && vite build）通过
- [x] 真实入口冒烟（商户成员 `ego` 登录，1280×480 视口强制侧栏溢出）：滚动条槽位 6px，轨道透明，拇指常驻且圆角；拇指悬停颜色由 `rgb(73,74,77)` 增亮到 `rgb(118,118,118)`；无溢出时不可见
- [x] 折叠态（64px）：滚动条槽位 6px，菜单图标中心 32px 与侧栏中心 32px 对齐，无偏移
- [x] 亮色主题下侧栏仍为石墨深壳，滚动条表现与暗色一致

## 风险与后果

- `overflow-y: scroll` 会常驻预留 6px 槽位：展开态菜单内容宽度比改前窄 6px；折叠态图标仍按 naive-ui 固定缩进居中（实测 32/32），不受影响
- 无溢出时滚动条槽位仍存在（透明不可见），属于以 6px 宽度换取溢出切换不抖动的取舍
- Firefox 的 `scrollbar-width: thin` 不支持圆角，拇指为细方条；颜色与透明轨道仍生效
- 后续深壳内其他独立滚动区如需同款样式，复用 `--sx-shell-scrollbar*` token 与相同伪元素规则，不要另造颜色

## 交叉链接

- 业务记忆：[../../../memory/business/2026-10-17-侧边栏滚动条优化.md](../../../memory/business/2026-10-17-侧边栏滚动条优化.md)
- 经验记录：[../../../memory/lessons/2026-10-17-chromium-scrollbar-custom-style.md](../../../memory/lessons/2026-10-17-chromium-scrollbar-custom-style.md)
- 前置约束：[2026-10-08-dark-mode-sider-shell-tokens.md](2026-10-08-dark-mode-sider-shell-tokens.md)
- 样式规范：[../../../frontend/frontend-rules.md](../../../frontend/frontend-rules.md)（样式规范节）
