<!-- last-updated: 2026-10-09 -->
# 租户 RBAC 本地化：角色/权限下沉本地，账号身份留平台

## 背景

2026-10-08 租户门户接入时，角色/权限经平台开放面实时消费（零本地投影）。随之而来的代价：13 端点/13 scopes 配置面、409 语义靠平台中文文案 Contains 匹配、权限码目录需启动 PUT 推送回平台（该路由平台侧从未注册，链路实际断链）、最后管理员守卫跨开放面翻 20 页、门户权限回收有 30-60s 缓存窗口。

## 决策

**认证留平台、授权本地**（与 B 端管理台「平台认证 + 本地 RBAC」同模式；平台仓配套变更见彼仓 `2026-10-09-tenant-rbac-localization`）：

- **本地三表**（tenant_id/user_id 均为平台 ID，无外键；账号身份事实源在平台）：
  - `tenant_roles`（(tenant_id, code) 唯一，软删+墓碑释放槽位）
  - `tenant_role_perms`（复合主键，替换物理删）
  - `tenant_user_role_binds`（(user_id, role_id) 复合主键）——**表名避开 `tenant_user_roles` 旧名**：该旧表（app_user 双角色时代）仍被 migrateLegacy 每次启动 HasTable→DropTable 清理，同名新表会被反复清空。
- **permcatalog.go 成为权限码目录唯一事实源**：补 `NormalizePerms`（目录精确码校验/去重/排序/上限 128）与 `PermAllowed`（移植自平台 perms.go）；启动 PUT 同步链路删除（连同平台侧未挂路由的缺陷一起消失）。新增权限码三件套纪律不变（注册表 + 路由常量 + 前端 i18n）。
- **TenantAuth 装载回调合并本地权限解析**：平台 profile（身份）→ `RoleRepo.ResolvePerms(user_id, tenant_id)`（binds→roles→role_perms JOIN，tenant 双重收敛防跨租户残留）→ PermCodes 随 TenantSubject 进 tnt: 缓存。权限变更 ≤60s 生效（与平台下发时同语义）；本地解析失败按空集保守处理（认证面 profile/改密不受影响）。RequireTenantPerm 零改动。
- **管理台**：角色 5 法 + 目录切本地 RoleUsecase（归属租户经本地租户投影校验）；用户 Create=平台建号+本地 binds、SetUserRoles=平台 GetUser 定位归属+本地替换（角色须同租户）、ListUsers=平台列表+本地 binds 批量补 RoleIDs。**响应 VO 形状不变，前端零改动**。
- **门户成员**：locateMember 改 `gw.GetUser(id)`+tid 校验（去 listAllUsers 扫描）；角色读写切本地；最后管理员守卫保留平台用户翻页（启停状态事实源）+本地权限映射（单查询）。
- **平台侧配套**：开放面 tenant-user 域收缩为身份面 6 scopes + 新增 `GET /open-api/v1/tenant-users/:id`（复用 `tenant-user:list`）；平台四表 DropTable；商户 scopes 前置 13→6（`tenant-role:*`/`tenant-user:setRoles` 退役）。

### 已知取舍

- 平台删号/停号后本地 binds 成孤儿——无害（账号进不来即无权限），零投影原则不做对账清理。
- 切换窗口（service 先部署读空本地表 → 迁移 SQL 灌数 → 平台部署清源表）角色页短暂空数据，仅 dev 受影响。
- 409 用户名文案匹配保留（账号仍在平台，开放面无机器可读子码——平台侧待改进项，另行立项）。

## 关键落点

- `internal/data/model/tenantrole.go`（三 PO）、`internal/data/tenantrole/repo.go`（仓储：墓碑/占用拒删/替换语义/ResolvePerms）
- `internal/biz/tenantuser/rolerepo.go`（端口+哨兵+PermResolver）、`roleusecase.go`（用例+视图）、`gateway.go`（收缩为用户 7 法）、`permcatalog.go`（Normalize/Allowed）
- `internal/server/middleware/middleware.go`（TenantAuth 增 perms 参数）、`internal/server/router.go`（装配）
- `internal/service/tenantuser/service.go`（组合）、`internal/service/tenantmember/service.go`（数据源切换）
- `internal/platformsdk/`（删角色域/同步、增 GetTenantUser）、`cmd/server/main.go`（删 syncTenantPermCatalog）
- `scripts/migrate-tenant-rbac.md`（一次性数据迁移；推荐顺序：部署 service → 灌数 → 部署平台）

## 验证

- `go build ./...` / `go test ./internal/... ./pkg/...` 全绿；wire 重新生成；前端 `vue-tsc -b` 零改动通过。
- 新增测试：`data/tenantrole/repo_test.go`（墓碑释放/占用拒删/替换语义/ResolvePerms 跨租户与去重/NormalizePerms）；`service/tenantmember/service_test.go` 重写（守卫切本地数据源 + 列表 RoleIDs 补齐）。
- 联调冒烟待 dev 双仓部署后执行（见计划 D 节清单）。
