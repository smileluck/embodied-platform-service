<!-- last-updated: 2026-10-16 -->
# 用户管理平台超管行锁定（消费 is_super 契约）

## 问题

用户管理页（`/system/users`，成员来源=平台本商户绑定成员）对「平台超级管理员」行没有任何
特殊处理：商户管理员可对超管行关闭准入（写回平台 `merchant_users.admitted`）、分配本地角色、
甚至移除成员（解绑）——平台唯一管理面入口的账号可被业务端误操作。此前开放面成员视图也
没有超管识别字段，本端无从判定「这行是超管」（只能硬编码平台用户 ID=1，契约脆弱）。

## 决策

平台侧同日先行落地契约（embodied-platform 1a0d42ba：开放面三端点超管 403 + 成员视图
`is_super` 标记），本端三层消费：

- **投影快照**：`platform_users` 表加 `is_super` 列（`Projection.IsSuper`），同步
  `SyncFromPlatform` 从开放面 `is_super` 刷新（含存量投影首次同步补齐）；成员列表行
  `MemberRow.IsSuper` 实时来自平台（页面锁定不依赖投影存在）。
- **接口限制（本地哨兵）**：`ErrPlatformSuperAdmin`（403）——`SetEnabledByPlatformUser` /
  `SetRolesByPlatformUser` / `DeleteMember` 命中超管投影一律拒绝，且**先于平台写回**
  （`SetEnabledByPlatformUser` 顺带把「平台先行」重排为「本地解析先行」：无投影行 404 发生在
  平台写入前，消除旧序「平台已改、本地 404」的半提交）；平台侧开放面守卫兜底直调（未同步/
  无投影场景，平台 403 经 SDK 错误透传）。本端成员管理无「编辑资料」操作（改昵称走本人
  `/auth/profile`），锁定面=开关准入/分配角色/移除三类。
- **页面隐藏**：`Users.vue` 超管行操作列不渲染（分配角色/开启/关闭准入/删除全部隐藏，
  空操作渲染「—」），角色列前置「平台超管」红色标签（`user.platformSuperAdmin`，zh/en）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 本地硬编码平台用户 ID=1 做守卫 | 把平台种子语义硬编码进 SDK 侧，契约变更（超管换 ID/多超管）即静默失效；走开放面标记，平台是唯一事实源 |
| 只靠平台侧 403，本端不加哨兵 | 本地角色分配（`PUT /users/:id/roles`）是纯本地操作不经平台，平台守卫够不着；且平台写回失败前已有半提交窗口 |
| 超管投影恒 Enabled=true 本地自愈 | 准入状态以平台为事实源（同步语义 `p.Enabled = a.Admitted`）；平台守卫后 `admitted` 不再可关，无需本地另立真相 |

## 验收标准

- [x] `go test ./internal/biz/admission/`（新增：超管三类操作拒绝且零平台写回、同步快照
  is_super 后守卫生效）
- [x] `go build ./...`、`vue-tsc --noEmit` 通过
- [ ] 联调：超管行按钮隐藏；直调 `PUT /users/1/admission` / `PUT /users/1/roles` /
  `DELETE /users/1` 返回 403 `平台超级管理员账号受保护…`

## 风险与后果

- `is_super` 列由 AutoMigrate 补列（`not null default false`），存量投影首次同步（手动或
  任务）前守卫按 false 处理——该窗口内平台侧守卫仍兜底 403，双层防线下无裸奔。
- 平台开放面契约新增字段：旧平台 + 新 service 组合（先升 service）无 `is_super` 字段，
  行为退化为现状（无锁定），升级平台后自愈——部署顺序仍是**先平台后 service**。

## 交叉链接

- 平台侧决策：`../embodied-platform/aiDoc/notes/implemented/feature/2026-10-16-openapi-superadmin-member-guard.md`
- 契约事实：embodied-platform `aiDoc/contracts/boundary.md`「开放面用户域=商户成员」条目
- 代码：`internal/biz/admission/{entity,usecase}.go`、`internal/data/admission/repo.go`、
  `internal/data/model/model.go`、`internal/server/handler_user.go`、`web/src/views/system/Users.vue`
- 业务需求：`aiDoc/memory/business/2026-10-16-超管成员保护-用户管理锁定.md`
