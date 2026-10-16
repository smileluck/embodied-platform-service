<!-- last-updated: 2026-10-16 -->
# 成员准入投影定时对账（admission_reconcile，免手动同步）

## 问题

平台管理面「准入接入方」把用户准入给本商户后，本服务侧毫无感知：新成员无本地投影 →
登录 403（`PlatformAuth` 无投影一律拒绝，唯一例外是平台标记管理员的懒建自愈）、
用户管理页该行无准入操作——必须有人手动点「从平台同步成员」才补建投影。
租户域早已由 `tenant_reconcile`（*/5）解决同构问题，成员准入域只有手动 `POST /users/sync`。

## 提案 / 决策

**定时对账（方案 A）**：复用既有 `SyncFromPlatform` 全量对账（补建缺失投影=默认准入、
快照/准入状态刷新、管理员标记→角色 2 对账、孤儿投影回收），新增 job handler
`admission_reconcile`（cron `*/5`，与 `tenant_reconcile` 同周期同模式）+ 种子任务
「成员准入投影对账」幂等播种。平台侧直接变更（准入/解绑/管理员标记/删号）≤5 分钟回流，
漂移上界与租户域一致。

配套加固（无人值守运行前提，手动时代不需要）：

1. **空结果护栏**：平台拉取成功但成员集为空时跳过回收阶段（角色回收+孤儿删除）——
   空结果无法区分「平台真无成员」与「列表接口异常返回空」，批量删投影不可逆
   （与 tenant_reconcile 同口径，宁可下轮非空真相再回收）。
2. **并发互斥**：`Usecase.syncMu` 串行化 `SyncFromPlatform`——job 层互斥只覆盖同任务
   调度触发（SkipIfStillRunning + running 标记），手动同步（HTTP）与定时任务重叠时会
   双拉双写交错（孤儿删除竞态可报 ErrNotFound）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| B：登录时自动建投影（把 `EnsureMerchantAdmin` 例外扩展到全部绑定成员） | `/auth/profile` 的 merchants 引用不带 `admitted` 字段——平台侧已关准入但本地无投影的边缘态会被误建为 Enabled=true；且把「无投影 403」闸门改成「绑定即放行」，准入关闭语义只剩平台侧单层。可作为后续契约演进（profile 下发 admitted）再评估 |
| C：平台推送（准入变更 webhook/事件通知） | 平台订阅转发域只覆盖设备/告警/数据事件，无用户域推送契约；需平台加事件类型 + 本服务暴露入站回调端点（现无任何平台→service 入站面），为 ≤5 分钟新鲜度引入双向新契约，收益不匹配 |
| 更高频对账（*/1） | 成员准入非高频变更场景；租户域 */5 已验证够用，缩短周期只放大开放面调用量 |

## 验收标准

- [x] 单测：空结果护栏保住本地投影与管理员角色（新增 `TestSyncFromPlatform_EmptyPlatformListSkipsRecycle`）
- [x] `go build ./...`、`go test ./...` 全绿（job 用例适配新构造参数）
- [x] 种子任务幂等播种（EnsureSeeded 按 Name 去重，存量库只补一行）
- [ ] 部署后观察：job_logs 出现「成员准入投影对账」成功记录；平台准入新成员 ≤5 分钟本端可登录

## 风险与后果

- 每 5 分钟一次开放面 `GET /open-api/v1/users` 全量分页拉取（与租户对账同量级，规模=本商户成员数/100 页）。
- 孤儿回收仍依赖平台非空真相：商户在平台侧解绑全部成员的极端场景，末位投影会滞留到
  下一轮非空拉取——与租户域同取舍，可接受。
- 手动「从平台同步成员」按钮保留（即时对账入口，任务只是兜掉人工记忆负担）。

## 交叉链接

- 同构先例：`aiDoc/notes/implemented/architecture/2026-10-05-tenant-reconcile-and-gate-status.md`
- 代码：`internal/biz/job/usecase.go`（handler/种子）、`internal/biz/admission/usecase.go`
  （syncMu + 空结果护栏）、`cmd/server/wire.go` + `wire_gen.go`（AdmissionReconciler 绑定）
- 业务需求：`aiDoc/memory/business/2026-10-16-成员准入定时对账.md`
