<!-- last-updated: 2026-10-10 -->
# 消息通知「按用户」发布：下拉打开恒空 + 定向用户名回填键错位

## 问题

用户反馈：消息通知页发布通知、送达范围选「按用户」时，用户列表为空。两层根因：

1. **前端**：用户多选框是 `remote` 远程搜索（`Notices.vue`），选项只在输入关键词后才请求；直接点开下拉不输入恒为空——后端 `kw` 空串本可返回前 20 个启用用户，前端从未发起这次请求。
2. **后端**（顺带发现的展示 bug）：`repo.userNames` 查询按 `platform_user_id IN (?)` 取行，但有昵称的分支却写 `out[row.ID]`——`row.ID` 是未被 Select 的本地自增主键（恒为 0），而调用方按 `platform_user_id` 查找。结果是**有昵称的用户**在通知列表/编辑回显的 `user_names` 中落到 `id#n` 占位符，且与 `user_ids` 序列错位；无昵称用户反而正确。

## 提案 / 决策

- 前端：`watch(form.scope)`，切到 `users` 且选项为空时以空关键词预载首屏（复用 `onSearchUsers('')`，后端 limit 缺省 20）。保留远程搜索语义，输入仍可模糊过滤。
- 后端：`userNames` 两个分支统一以 `row.PlatformUserID` 为键；昵称展示格式对齐前端选项口径「昵称（用户名）」（全角括号）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 去掉 remote、全量拉用户做本地过滤 | 用户量无上界（准入投影可增长），远程搜索是既有正确设计，只是缺首屏 |
| 选项数据源改为实时拉平台成员（同用户管理页） | 通知目标应限定本系统已准入（enabled）用户——未准入者无法登录本控制台，收到通知也无意义；本地准入投影是刻意口径（repo 注释明示） |

## 验收标准

- [x] `go test ./internal/data/notice/...` 通过（含 ListUserOptions 既有用例）
- [x] `cd web && npx vue-tsc -b` 通过；改动文件 eslint 无 error
- [ ] 手工冒烟：发布弹窗切「按用户」→ 下拉立即出现启用用户；发布后列表/编辑回显 user_names 为「昵称（用户名）」而非 `id#n`（待用户环境验证）

## 风险与后果

- 预载仅在选项为空时触发一次，不改变搜索防抖行为。
- 若环境内除当前管理员外确无已准入成员，下拉仍只有管理员一人——那是数据状态（需在用户管理页「同步成员」或开启准入），不是本修复范围。

## 交叉链接

- 代码：`web/src/views/system/Notices.vue`（scope watch 预载）、`internal/data/notice/repo.go` `userNames`
- 经验：`aiDoc/memory/lessons/2026-10-10-nselect-remote-no-preload.md`
