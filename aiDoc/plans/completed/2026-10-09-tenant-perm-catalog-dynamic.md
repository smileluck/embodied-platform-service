<!-- last-updated: 2026-10-09 -->
# 租户门户权限点目录动态化（商户端注册表 → 平台按商户目录）

> 路径：`aiDoc/plans/completed/2026-10-09-tenant-perm-catalog-dynamic.md`（已归档）

## 目标

门户权限码（`device:*`/`member:*` 等）的事实源从平台静态 `catalog`（`embodied-platform/internal/biz/tenantuser/perms.go`）迁到商户端注册表（本仓 `internal/biz/tenantuser/permcatalog.go`）；平台目录动态化：

1. 平台新表 `tenant_perm_defs`（AutoMigrate）按 `(merchant_id, code)` 存商户注册码，保留 27 个静态码作基础集（base）
2. 平台开放面新增 `PUT /open-api/v1/tenant-user-perms`（scope `tenant-role:syncPerms`）：商户端全量 upsert 注册，**只增不删**（幂等；回滚安全）
3. 目录读取按商户收敛：开放面 `GET /tenant-user-perms` 返回 base ∪ 本商户注册码；角色创建/更新的 `NormalizePerms` 校验同口径
4. 本仓：权限码收敛为注册表常量（router 挂载引用常量，编译期防拼写错误）；启动时异步向平台推送注册表（重试至成功，不阻断启动）
5. 商户端新增门户权限码 = 注册表加一行 + 挂路由 + 前端 i18n 文案，**发版即生效，无需平台改代码**

## 非目标

- 不做注册码删除/治理（废弃码留存无害；治理留平台管理面后续）
- 平台管理面前端不改（目录接口支持可选 `tenant_id` 参数过滤；不传返回 base ∪ 全部注册码并集，兜底入口兼容）
- 不改门户鉴权语义（`RequireTenantPerm` 精确匹配、`tnt:` 自省缓存不变）
- 不动商户开放面 scope 域通配（`reservedDomains`）机制
- 本期不新增业务权限码（27 个 base 原样迁移对齐）

## 假设

- 平台 AutoMigrate 开启（`db.autoMigrate`），新表随部署自动建
- 商户 scope 为部署时配置数据（非代码种子），`tenant-role:syncPerms` 需 deploy 前置补配（对齐 `tenant-user:*`/`tenant-role:*` 先例）
- 角色权限码存 `tenant_role_perms` 关联表（字符串、无外键），存量角色不受目录动态化影响
- `TenantSnapshot` 增加 `MerchantID` 字段对现有 JSON 消费方向后兼容（多字段无害）

## 影响面

- **平台仓** `embodied-platform`：`../embodied-platform/internal/biz/tenantuser`（perms/repo/usecase/entity）、`../embodied-platform/internal/data/model/tenantuser.go`、`../embodied-platform/internal/data/tenantuser/repo.go`、`../embodied-platform/internal/data/data.go`（AutoMigrate）、`../embodied-platform/internal/biz/merchant/scopes.go`、`../embodied-platform/internal/server/openapi.go`、`../embodied-platform/internal/server/openapi_tenantuser.go`、`../embodied-platform/internal/service/openapi/tenantuser.go`、管理面 `service/tenantuser`+`handler_tenantuser.go`、`../embodied-platform/cmd/server/wire.go`、`../embodied-platform/sdk/`
- **本仓**：`internal/biz/tenantuser/permcatalog.go`（新）、`internal/server/router.go`（常量化）、`internal/platformsdk/{client,types}.go` + 测试、`cmd/server/main.go`（启动推送）
- **契约**：开放面新增 PUT 端点 + 目录 GET 返回内容变化（base→base∪注册码）；SDK 双仓同步（先平台后本仓纪律）
- 前端（两仓）零改动

## 验收标准

- [ ] 平台 `go build ./... && go vet ./... && go test ./...` 通过（含 usecase 动态目录用例、sdk 对账测试）
- [ ] 平台 `make wire` 重新生成 wire_gen.go（禁手改）
- [ ] 本仓 `go build ./... && go vet ./... && go test ./...` 通过（含 platformsdk 对账测试）
- [ ] 目录合并语义：base 顺序保留 + 注册码按 (group, code) 稳定追加；注册码撞 base 码时 upsert 幂等忽略
- [ ] 角色校验按租户归属商户目录（开放面/管理面同口径）；存量 base 码角色不受影响
- [ ] 本仓 router 无权限码字面量残留（全部引用注册表常量）
- [ ] 双仓决策记录 + business 记忆 + aiDoc 同步完成

## 工作项

| # | 工作项 | owner | 依赖 | 建议写范围 | 验证命令 | 状态 |
|---|---|---|---|---|---|---|
| 1 | 平台：TenantPermDefPO + AutoMigrate + Snapshot.MerchantID | AI | - | `../embodied-platform/internal/data/model/tenantuser.go`、`../embodied-platform/internal/data/data.go`、`../embodied-platform/internal/data/tenantuser/repo.go` | `go build ./...` | 未开始 |
| 2 | 平台：biz 目录动态化（MergeCatalog/ValidatePermDef/NormalizePermsIn/PermRepo/SyncPerms/PermCatalogFor） | AI | 1 | `internal/biz/tenantuser/{perms,repo,usecase,entity}.go` | `go test ./internal/biz/tenantuser/...` | 未开始 |
| 3 | 平台：scope + 开放面 PUT 端点 + 目录 GET 按商户 + 管理面目录过滤 + usecase 角色校验接目录 | AI | 2 | `scopes.go`、`openapi*.go`、`service/openapi/tenantuser.go`、`service/tenantuser`、`handler_tenantuser.go` | `go build ./... && go vet ./...` | 未开始 |
| 4 | 平台：wire 重生成 + 测试补齐 | AI | 3 | `../embodied-platform/cmd/server/wire.go`、`usecase_test.go` | `make wire && go test ./...` | 未开始 |
| 5 | 平台：sdk SyncTenantUserPerms + 对账测试 + 决策记录 + aiDoc 同步 | AI | 3 | `sdk/{client,types,client_test}.go`、`aiDoc/` | `go test ./sdk/...` | 未开始 |
| 6 | 本仓：platformsdk 同步（client/types/test） | AI | 5 | `internal/platformsdk/` | `go test ./internal/platformsdk/...` | 未开始 |
| 7 | 本仓：permcatalog 注册表 + router 常量化 | AI | - | `internal/biz/tenantuser/permcatalog.go`、`internal/server/router.go` | `go build ./... && grep` | 未开始 |
| 8 | 本仓：main.go 启动推送 goroutine | AI | 6,7 | `cmd/server/main.go` | `go build ./... && go vet ./...` | 未开始 |
| 9 | 本仓：决策记录 + business 记忆 + lessons + 漂移自检 | AI | 1-8 | `aiDoc/` | `check_sync` / 路由表核对 | 未开始 |

## 验证命令

```bash
# 平台仓
cd ../embodied-platform && go build ./... && go vet ./... && go test ./... && make wire
# 本仓
go build ./... && go vet ./... && go test ./...
# 契约对账
grep -rn "RequireTenantPerm(\"" internal/server/router.go  # 应零命中
```

## 回滚 / 迁移

- 无存量数据迁移：新表空启动即等价旧行为（目录=base）
- 回滚本仓推送：撤 main.go goroutine 即可（平台注册码留存无害）
- 回滚平台：部署旧版（表留存无消费方）；`NormalizePerms` 回到静态目录语义——已注册商户码的角色保存会开始被拒，需先降级商户端

## 当前状态

全部工作项完成（2026-10-09 11:40）。执行顺序按 10:34 调整：本仓侧先行完成并验证，平台侧待并行工作（dataset 开放面移除 + 数据映射开放面）收尾后叠加实施完毕。

## 交付摘要（完成时填写）

**代码交付**：

- 本仓：`internal/biz/tenantuser/permcatalog.go`（27 码注册表事实源，含 dataset 域）；`internal/server/router.go` 门户路由全部改引常量（9 处字面量清零）；`internal/platformsdk` 补 `SyncTenantUserPerms` + `TenantUserPermSyncRequest` + 对账测试；`cmd/server/main.go:syncTenantPermCatalog` 启动异步推送（30s 重试至成功、4xx 永久错误放弃并提示 scope 配置）
- 平台仓：`tenant_perm_defs` 表 + `permRepo`；biz 目录动态化（`MergeCatalog`/`NormalizePermsIn`/`ValidatePermDef`/`SyncPerms`/`PermCatalogFor(ForTenant)(All)`）；`TenantSnapshot.MerchantID`；开放面 `PUT /open-api/v1/tenant-user-perms`（scope `tenant-role:syncPerms`）+ GET 按商户收敛 + 管理面目录 `tenant_id` 过滤；`../embodied-platform/sdk/` 同步；scope 目录 + 集成指南 + boundary 同步

**验证结果**：

- 本仓 `go build/vet` + `go test ./...`：passed（含 `TestClient_TenantUserPermSync`）
- 平台主模块 `go build/vet` + `go test -count=1 ./...`：passed（含动态目录 5 用例）
- 平台 `cd sdk && go build/vet/test -count=1`：passed（含 `TestClient_TenantUserPermSync`）
- 平台 `check_sync.py`：9 通过 / 0 失败
- 联调冒烟：**not-run**（待 deploy 环境；前置=商户配 `tenant-role:syncPerms` scope、平台先行部署）

**与计划的偏差**：

- 执行顺序反转（平台仓并行工作在途，本仓先行），契约两侧最终对齐无偏差
- 平台 base 目录为 23 码（dataset 4 码已被同日 dataset 移除工作撤下，注册表 27 码由本仓带回，合并后与原 27 码目录等价）
- 平台仓并行会话（data-mapping）的 wire 重生成顺带吸收了本变更加的 PermRepo 依赖；其 dataset 移除提交（dc929b07）混入了本变更的 perms.go/scopes.go 部分——平台仓现有三批未提交/已提交混合工作，建议用户按域分批提交

**遗留事项**：

- 联调冒烟（deploy 前置：平台先行部署 + 商户 scope 补配）
- 废弃注册码的平台管理面治理入口（后续）
- 平台管理面前端目录接口按 `tenant_id` 过滤消费（接口能力已具备，前端未接）
