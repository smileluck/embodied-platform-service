<!-- last-updated: 2026-10-09 -->
# 租户门户权限点目录动态化（商户端注册表 → 平台按商户目录）

> 路径：`aiDoc/notes/implemented/architecture/2026-10-09-tenant-perm-catalog-dynamic.md`

## 问题

租户门户权限码（`device:*`/`member:*` 等）的功能开发全部发生在商户端（本服务）——权限码定义、路由挂载（`RequireTenantPerm`）、前端文案都在本仓。但配权 UI 的合法码目录是平台仓静态代码（`embodied-platform/internal/biz/tenantuser/perms.go` 的包级 `catalog`，创建/改角色时平台 `NormalizePerms` 强校验）。结果是每新增一个门户权限码都要双仓协调：平台改目录发版、商户端改路由发版，且平台目录里混入了商户端业务域（dataset），违背「平台=基础设施、商户业务自治在 service」的分工。

## 提案 / 决策

**权限码事实源迁到商户端注册表，平台目录动态化（base ∪ 商户注册码，按商户隔离，只增不删）**（用户选定路线二）：

1. **本仓注册表**：`internal/biz/tenantuser/permcatalog.go`——27 码常量 + `Catalog`（初始集对齐平台 2026-10-08 静态目录，含平台 2026-10-09 已撤下的 dataset 域 4 码，由本表带回）。router 挂载从字符串字面量改为引用常量（编译期防拼写错误）。
2. **同步机制**：服务启动异步推送（`cmd/server/main.go:syncTenantPermCatalog`，对齐 `checkPlatform` 先例）——`PUT /open-api/v1/tenant-user-perms`（scope `tenant-role:syncPerms`，幂等、只增不删）；网络/5xx 每 30s 重试至成功，4xx（scope 未配/格式拒绝）为永久错误记 error 放弃。
3. **平台侧**：新表 `tenant_perm_defs`（`(merchant_id, code)` 唯一，AutoMigrate）；开放面 GET 目录按签名商户收敛返回 base ∪ 本商户注册码；角色创建/更新的 `NormalizePerms` 按「租户归属商户目录」校验；管理面目录接口支持 `tenant_id` 参数过滤（不传=全景并集，平台运营兜底）；`sdk/` 补 `SyncTenantUserPerms`（先平台后本仓同步纪律）。
4. **新增权限码三件套同发版**：注册表一行 + 路由挂常量 + 前端 i18n 文案（`tenantRole.perm/permGroup`），无需平台改码。

**否决的备选**：①维持双仓静态同步+对账测试（不解决协调成本）；②平台放弃目录强校验、商户端完全自治（平台管理台兜底入口退化为手输码、拼写错误无拦截、平台三面校验口径分裂）。纯「本地扫描+缓存」不可行——角色落库在平台，绕不开平台目录校验，必须平台侧配合。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 双仓静态目录 + 契约对账测试 | 零平台改动，但加码仍需平台发版，协调成本不变，只防漂移不治本 |
| 平台放宽为格式校验（商户端自治目录） | 平台管理台兜底配权无目录可勾；废弃码/拼写错误无治理；平台门户面与管理面校验口径分裂 |
| 源码 AST 扫描权限码 | 脆弱且需 codegen；常量注册表 + 编译期引用更可靠 |
| Redis 缓存目录 | 码表只随发版变化，进程内即天然缓存；当前透传拉取成本极低 |

## 验收标准

- [x] 本仓 `go build/vet/test` 通过（含 `TestClient_TenantUserPermSync` 契约对账：PUT /open-api/v1/tenant-user-perms + body 序列化）
- [x] 本仓 router 无权限码字面量残留（全部引用 `biztenantuser.Perm*` 常量；`tenantmember` 的 `member:` 前缀判定与 B 端管理权限种子属其他命名空间，不在此列）
- [x] 平台 `go build/vet` + `go test ./...` + `cd sdk && go build/vet/test` 通过（含 usecase 动态目录 5 用例、sdk 对账测试）
- [x] 目录合并语义：base 顺序保留 + 注册码按 (group, code) 稳定追加；注册码撞 base 码幂等忽略
- [x] 角色校验按租户归属商户目录（`TestCreateRole_DynamicPermScope`：本商户码放行、他商户码拒绝、base 码放行）；存量角色不受影响（perm_codes 存关联表无外键）
- [ ] 联调冒烟：本仓启动推送成功 → 角色配权可勾选注册码 → 门户带新码角色访问放行（待 deploy 环境；前置=商户配 `tenant-role:syncPerms` scope、平台先行部署）

## 风险与后果

- **部署前置**：商户须配 `tenant-role:syncPerms` scope（第 13 项，对齐 tenant-user:*/tenant-role:* 先例），否则启动推送 403（日志明确提示）；平台先行部署。
- **只增不删**：商户端回滚版本不会收缩平台目录（防目录塌缩误伤存量角色）；废弃码治理留平台管理面后续。
- **多商户同码**：各商户注册各自隔离；平台管理面全景目录对同码去重（先见者为准）。
- **平台 base 目录仍是下限**：平台静态码与商户注册码并存（撞码幂等忽略），平台保留跨商户通用码兜底。
- **同日并行变更**：平台仓同日有「dataset 开放面移除」（已撤 dataset 域）与「数据映射开放面」在途工作，本变更叠加其上实施；本仓注册表带回 dataset 域即是对前者的衔接。

## 交叉链接

- 变更计划：`aiDoc/plans/completed/2026-10-09-tenant-perm-catalog-dynamic.md`（含平台仓并行工作调度）
- 前置：`aiDoc/notes/implemented/architecture/2026-10-08-tenant-portal-tenantusers-migration.md`（租户用户/角色管理面接入，静态目录的来由）
- 平台侧：`embodied-platform/aiDoc/notes/implemented/architecture/2026-10-09-tenant-perm-catalog-dynamic.md`（平台侧决策：表/端点/目录合并/校验口径）
- 关键代码：`internal/biz/tenantuser/permcatalog.go`、`cmd/server/main.go:syncTenantPermCatalog`、`internal/platformsdk/client.go:SyncTenantUserPerms`、`internal/server/router.go`（常量挂载）
