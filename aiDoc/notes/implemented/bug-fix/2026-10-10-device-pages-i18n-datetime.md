<!-- last-updated: 2026-10-10 -->
# 设备页面英文枚举裸显与 RFC3339 原串时间：状态 i18n 映射 + formatDateTime 统一

## 问题

用户反馈两项：① 设备管理界面「激活状态」列显示英文原文（inactive/active/disabled/retired）；② 设备详情 tab 内未正确 i18n、创建时间未按统一格式显示。排查：tab 标签本身已正确接 i18n（key 双语齐备），实际裸显的是**tab 内容与页面各处的英文枚举**——详情头部状态 LED、命令 tab 的命令状态（pending/dispatched/acked/…）、租户门户两页的状态下拉/列/详情。时间方面：设备接口经平台开放面代理返回 RFC3339 原串（`2026-10-10T01:39:01Z`），`created_at`/`last_seen_at`/遥测 `ts`/事件 `occurred_at` 均未走 `utils/datetime.ts` 的 `formatDateTime`（站内统一 `YYYY-MM-DD HH:mm:ss`，文件/数据映射页是正确范例）。

## 提案 / 决策

- 状态枚举展示一律经映射函数（未知值兜底显原值）：管理端复用既有 `device.statusXxx` key；租户门户在 `tenantPortal.devices` 补 4 个状态 key（zh/en 同步），门户页不复用管理端命名空间。
- 命令状态在 `device.*` 新增 9 个 `cmdStatusXxx` key（zh/en 同步），命令列、刷新 toast 同一映射。
- 所有设备页时间字段套 `formatDateTime`（空值显示 `—`）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 租户门户直接引用 `device.statusXxx` key | 门户页全部文案在 `tenantPortal.*` 命名空间，跨命名空间引用破坏归属一致性 |
| 后端代理层把时间转格式、枚举译好再下发 | 展示口径属于前端职责；后端 formatTime 只覆盖本服务自产接口，平台代理链保持透传契约 |

## 验收标准

- [x] `cd web && npx vue-tsc -b` 通过；改动文件 eslint 无 error（仅既有 warning）
- [x] zh-CN / en-US 语言包 key 同一次变更成对新增
- [ ] 手工冒烟：设备列表/详情（管理端 + 租户门户）状态、命令状态显示中文；创建时间显示 `YYYY-MM-DD HH:mm:ss`（待用户环境验证）

## 风险与后果

- `Devices.vue`（管理端）columns 为非响应式常量（既有模式），语言切换后需重载数据才刷新列文案——本次沿用现状，不扩大改动面。
- 「tab 未翻译」的直接诱因已确认：Go 服务托管 `web/dist`（`router.go` registerStatic），修复落地时 dist 仍是 10-09 旧构建。2026-10-10 已重新 `npm run build`（5.71s，仅既有 chunk 体积警告）并抽查产物含全部状态/命令状态文案；静态目录按请求读盘，无需重启后端，浏览器强刷即可。

## 交叉链接

- 代码：`web/src/views/device/Devices.vue`、`web/src/views/device/DeviceDetail.vue`、`web/src/views/tenant-portal/Devices.vue`、`web/src/views/tenant-portal/DeviceDetail.vue`、`web/src/locales/{zh-CN,en-US}/{device,tenantPortal}.ts`、`web/src/utils/datetime.ts`
- 规则：`aiDoc/frontend/frontend-rules.md`「全部用户可见文案走 vue-i18n」（本次为回归修复）
