<!-- last-updated: 2026-10-09 -->
# 租户 RBAC 本地化：角色/权限下沉本地，账号身份留平台

> 需求权威源：会话批准计划（emb 项目集，2026-10-09）。配套平台仓计划见 `embodied-platform/aiDoc/plans/completed/2026-10-09-tenant-rbac-localization.md`。

## 目标

租户门户「认证留平台、授权本地」落地到本仓：

- 本地三表承接租户 RBAC：`tenant_roles`（tenant_id=平台租户 ID，(tenant_id,code) 唯一+墓碑）、`tenant_role_perms`（复合主键替换）、`tenant_user_role_binds`（(user_id=平台 tenant_user ID, role_id)；**避开 `tenant_user_roles` 旧名**——migrateLegacy 仍每次启动 DropTable 该旧表）。
- `permcatalog.go` 成为权限码目录唯一事实源（补 NormalizePerms/PermAllowed，移植自平台 perms.go）；删除启动 PUT 同步链路（连同平台侧 P0 缺失的路由一起消失）。
- TenantAuth 缓存装载回调 = 平台 profile（身份）+ 本地权限 JOIN，合并进 TenantSubject 进 tnt: 缓存（权限变更 ≤60s 生效语义不变）；RequireTenantPerm 零改动。
- 商户管理台：角色 5 法 + 目录切本地 RoleUsecase；用户 Create=平台建号+本地 binds、SetUserRoles=GetUser 校验+本地替换、ListUsers=平台列表+本地 binds 补 RoleIDs——**响应 VO 形状不变，前端零改动**。
- 门户成员：locateMember 改 `gw.GetUser(id)`+tid 校验（去 listAllUsers 扫描）；角色读写切本地；最后管理员守卫保留平台用户翻页（状态事实源）+本地权限映射。
- gateway 收缩为用户 7 法（+GetUserByID）；platformsdk 删角色域/同步、用户 Create 去 role_ids、新增 GetTenantUser（对应平台新增开放面 `GET /open-api/v1/tenant-users/:id`）。

## 非目标

- 平台仓改动（彼仓计划承接；切换顺序：迁移 SQL → 平台先部署 → 本仓部署）。
- 商户管理台/门户前端页面改动（后端响应形状保持不变）。
- 门户业务端点（设备只读/成员自治）功能不变，仅换数据源。

## 假设

- 平台开放面 tenant-user 域收缩为 6 端点/6 scopes 并新增 GET /:id（scope 复用 tenant-user:list），已同步彼仓计划。
- 开发库为唯一存量数据源；角色数据经一次性 SQL 迁移（scripts/migrate-tenant-rbac.sql，角色 id 原值保留）。
- 用户启停状态事实源在平台（本地 binds 不存状态）；最后管理员守卫按平台 status + 本地权限映射判定。

## 影响面

- 数据层：`internal/data/model/`（新 tenantrole PO）、`internal/data/tenantrole/`（新 repo）、`internal/data/data.go`（AutoMigrate 注册）。
- biz 层：`internal/biz/tenantuser/`（Gateway 收缩 + RoleRepo/RoleUsecase + permcatalog 补函数）。
- server 层：`middleware.go`（TenantAuth 装载回调）、`handler_tenantuser.go`、`router.go`（装配增参）。
- service 层：`internal/service/tenantuser/`（角色本地化/用户组合）、`internal/service/tenantmember/`（数据源切换）。
- SDK：`internal/platformsdk/client.go` + `types.go`；`cmd/server/main.go`（删 syncTenantPermCatalog）；wire。

## 验收标准

- [ ] `go build ./...` 通过；`go test ./internal/... ./pkg/...` 通过；wire 重新生成
- [ ] 本地三表 repo 测试：墓碑释放槽位、占用拒删 409、perms/binds 替换语义、ResolvePerms JOIN
- [ ] tenantmember 守卫用例切本地数据源后全绿
- [ ] 前端 `vue-tsc -b` 通过（预期零改动即通过）

## 工作项

| # | 工作项 | owner | 依赖 | 建议写范围 | 验证命令 | 状态 |
|---|---|---|---|---|---|---|
| 1 | 本地三表 PO + AutoMigrate + data/tenantrole repo | AI | 无 | internal/data/{model/,tenantrole/,data.go} | `go test ./internal/data/...` | 未开始 |
| 2 | biz：RoleRepo 端口 + RoleUsecase + permcatalog 补 Normalize/Allowed + Gateway 收缩 | AI | 1 | internal/biz/tenantuser/ | `go test ./internal/biz/...` | 未开始 |
| 3 | TenantAuth 本地权限解析 + wire/DI | AI | 1,2 | internal/server/middleware/、cmd/server/ | `go build ./...` | 未开始 |
| 4 | 管理台后端切换 + handler | AI | 2 | internal/service/tenantuser/、internal/server/handler_tenantuser.go | `go build ./...` | 未开始 |
| 5 | 门户成员切换 + gateway impl 收缩 | AI | 2 | internal/service/tenantmember/、internal/data/platform/ | `go test ./internal/service/...` | 未开始 |
| 6 | platformsdk 收缩 + main.go 删同步 | AI | 无 | internal/platformsdk/、cmd/server/main.go | `go test ./internal/platformsdk/...` | 未开始 |
| 7 | 测试补齐 + 文档收口（决策记录/boundary/memory/lessons）+ migrations SQL 清理 | AI | 全部 | aiDoc/、migrations/ | 全量验证命令 | 未开始 |

## 验证命令

```bash
go build ./...
go test ./internal/... ./pkg/...
export PATH=$PATH:$(go env GOPATH)/bin && make wire
cd web && npx vue-tsc -b
```

## 回滚 / 迁移

- 数据迁移：`scripts/migrate-tenant-rbac.md`（平台库三表 → 本库三表，幂等先清后插，附行数校对）；回退=删本地数据用新管理台重建。
- 代码回滚 = git revert；本地三表留存无害；切换顺序=迁移 SQL → 平台部署 → 本仓部署（两部署间角色页短暂 404，仅 dev 受影响）。

## 当前状态

全部工作项完成（2026-10-09）。

## 交付结果摘要（2026-10-09）

- 本地三表 + `data/tenantrole` 仓储（墓碑释放/占用拒删/替换语义/ResolvePerms 跨租户收敛）+ AutoMigrate 注册。
- biz：`rolerepo.go`（端口/哨兵/PermResolver）+ `roleusecase.go`（角色 CRUD/用户绑定/目录，归属租户经本地投影校验）+ permcatalog 补 NormalizePerms/PermAllowed + Gateway 收缩为用户 7 法（含 GetUser）。
- TenantAuth 装载回调合并本地权限解析（失败按空集保守）；身份代理/适配器/主体注释同步（profile 不再取 perm_codes）。
- 管理台 service：角色切本地、用户 Create/SetUserRoles/ListUsers 组合本地 binds，VO 形状不变（前端零改动，vue-tsc 零改动通过）；门户成员：locateMember 改 GetUser 单查、守卫与列表切本地权限映射。
- platformsdk 镜像平台收缩（删角色域/同步、增 GetTenantUser、Create 去 role_ids、scopes 常量 13→6）；main.go 删 syncTenantPermCatalog；wire 增 tenantrole.Repo/RoleUsecase 绑定。
- 测试：tenantrole repo 测试（墓碑/占用/替换/ResolvePerms/Normalize）、tenantmember 守卫测试重写、sdk 同步用例删除；`go build`/`go test ./internal/... ./pkg/...` 全绿。
- 迁移：`scripts/migrate-tenant-rbac.md`（推荐顺序：部署 service → 灌数 → 部署平台）；migrations 三方言删 tenant_user_roles/app_user_tenants 残留并补本地三表 DDL。
- 文档：决策记录 + 本计划归档 + lessons（binds 表命名坑）。
- 偏离计划处：无。联调冒烟待双仓部署后执行。
