<!-- last-updated: 2026-10-08 -->
# 深色模式侧栏白底白字——补深壳 token，纠正随主题反转变量的误用

## 问题

深色模式下管理端侧栏不可读：`.sider` 背景用了 `var(--sx-ink)`，而 `--sx-ink` 是"主文字"变量（亮 `#151E2B` / 暗 `#E6EBF2`），深色模式下侧栏底翻成近白；菜单配色 `menuOverrides`（米白文字 + 琥珀高亮）与 logo/脚注均为石墨深底设计 → 白底白字。同批问题：`--sx-shell-line/text/muted` 被多处使用但从未定义（靠 CSS 兜底行为偶然可显示）；`.seal`/`.avatar`（恒浅底 `--sx-accent-bright`）文字用 `--sx-ink`，深色下浅字浅底。

## 提案 / 决策

在 `variables.css` `:root` 新增亮暗不变的深壳 token 组（色值与 menuOverrides 暖白色系同源）：`--sx-shell`（石墨深壳底 `#151E2B`，与亮色原值一致、零亮色回归）、`--sx-shell-line/text/muted`。`.sider` 背景改用 `--sx-shell`；`.seal`/`.avatar` 文字改 `--sx-panel`（恒浅底配恒深字）。顶栏（非深壳、背景透出 `--sx-bg`）四处误用 `--sx-shell-*` 的地方改回语义正确的自适应变量：header 边线与 user-chip 边框 `--sx-line`、触发器图标 `--sx-muted`、用户名 `--sx-ink`——同一个 token 无法同时服务深壳与浅底，按实际底色分流。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 只把 `--sx-shell-*` 定义出来不改 `.sider` 背景 | 侧栏核心 bug 在背景变量误用，token 不补背景仍翻白 |
| 给顶栏也铺深壳背景（`--sx-ink-soft` 补深色定义） | 亮色模式顶栏观感大变，超出修 bug 范畴变成重设计；顶栏现状（透明透出 bg、inherit 兜底）可显示 |
| `--sx-shell` 取 tokens.css 深壳代码块的 `#0F1621` | 亮色模式侧栏会比现状变深，引入无必要的视觉回归；取 `#151E2B` 亮色零变化 |

## 验收标准

- [x] 深色模式：侧栏为石墨深底、米白/琥珀菜单可读，logo 与脚注文字可见
- [x] 亮色模式：侧栏、顶栏视觉与改前一致（`--sx-shell` 值即原亮色 `--sx-ink` 值）
- [x] 深色模式：印章/头像恢复深字浅底
- [x] `npx vue-tsc -b` 零错误、`npm run lint` 零 error

## 风险与后果

- 已知遗留（本次未动，非侧栏范围）：`.header` 背景 `var(--sx-ink-soft)` 未定义 → 透明透出 `--sx-bg`，若后续要顶栏独立底色需补定义或改 token；`--sx-ok`/`--sx-warn`/`--sx-ok-bright`（状态灯、登录页）同样未定义，靠兜底显示。
- 顶栏触发器图标由继承 `--sx-ink` 改为显式 `--sx-muted`，亮色下略淡（次级操作的正确语义）。

## 交叉链接

- 代码：`web/src/styles/variables.css`（深壳 token）、`web/src/layout/AdminLayout.vue`（.sider/.seal/.avatar/顶栏四处）
- 约束回写：`aiDoc/frontend/frontend-rules.md` 样式规范（恒定底色必须用亮暗不变 token）
