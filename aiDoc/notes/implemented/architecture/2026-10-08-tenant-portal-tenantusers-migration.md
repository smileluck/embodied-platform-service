<!-- last-updated: 2026-10-08 -->
# 租户门户身份切换到平台 tenant_users + 租户用户/角色管理面接入

> 路径：`aiDoc/notes/implemented/architecture/2026-10-08-tenant-portal-tenantusers-migration.md`

## 问题

平台侧 2026-10-08 落地了第四套身份 `tenant_users`（租户门户运营账号：单租户绑定 + 租户作用域 RBAC，三面=平台管理面/`/tenant-api/v1` 门户认证面/开放面 tenant-user+tenant-role 域），并在设计上明确否决了「复用 app_users 加角色列」。而本服务的租户门户（2026-10-08 上午落地）恰恰是「app 用户 token + 本地 `tenant_user_roles` 双角色」——两套租户门户成员模型并存且重复。同时商户侧缺少租户用户的管理入口：门户运营账号的增删改密若全部收口到平台管理台，则违背「平台=基础设施不含业务运营、商户自治在 service」的分工（多商户下平台运营成瓶颈）。

## 提案 / 决策

**管理入口=service 商户自管（经开放面，本地无投影）；门户身份一并切到平台 tenant_users，本地授权层退役**（用户逐项确认）：

1. **管理面**：service 管理台新增「租户用户/租户角色」两页（租户中心下），经开放面 `/open-api/v1/tenant-users*`（7 端点）+ `/tenant-roles*`（5 端点）+ `/tenant-user-perms`（目录，平台侧为本接入新增）实时消费，不建本地表。权限点种子 `tenantUser:*`/`tenantRole:*`（camelCase 对齐平台管理面）；租户下拉仅列已同步租户（platform_id>0）；角色配权用平台目录分组勾选。
2. **门户身份切换**：门户登录改经本服务后端代理平台 `/tenant-api/v1/auth/{login,refresh}`（对齐 B 端 2026-10-08 代理模式，服务端挂 LoginIPGuard+限流；token 双用）；新中间件 `TenantAuth` 自省平台 `GET /tenant-api/v1/profile`（`tnt:` 缓存 30-60s，与 B/C 端同一权衡），tid 取自 token（单租户绑定，**不再信任 X-Tenant-ID 头**）+ 本地租户闸门（未同步/停用即 403）；`RequireTenantPerm` 对自省 perm_codes 精确匹配（member:*/device:* 按路由粒度挂载，对齐平台目录）。
3. **门户成员自治**：`/tenant-api/v1/members*` 重写为开放面 tenant-user 域的 tid 锁定代理（`service/tenantmember` 重写，`tenant_id` 由认证主体注入）；守卫保留在 service 层——本人不可自移除/自禁用/自改角色；「最后一名成员自治管理员失格」拒绝（管理员判定=启用中且任一角色含 `member:*` 权限码，覆盖 tnt: 缓存窗口内权限被剥后的竞态）。
4. **本地授权层退役**：`tenant_user_roles` 表 + `biz/tenantmember` 旧实现 + `TenantAdmin` 中间件 + B 端应用用户页的「租户端角色」特性（创建时 tenant_role、行内设置、列表标注）全部删除（migrateLegacy 幂等 DropTable，对齐 app_users 先例）；`/app-api/v1` 收窄回 C 端探针（profile/改密），门户成员/设备端点全部迁至 `/tenant-api/v1`。
5. **前端**：`api/tenant.ts` baseURL 切 `/tenant-api/v1`（去 X-Tenant-ID）、`stores/tenantUser` 登录走服务代理（无租户切换概念）、菜单/路由守卫按 `perm_codes`（`isAdmin`=`member:user:list`）、vite 代理补 `/tenant-api`。
6. **SDK**：平台 `sdk/` 补 tenant-user 7 方法 + tenant-role 5 方法 + 目录方法（含 `client_test.go` 路径/方法/请求体对账测试，为该域首个真实消费方验证）；`internal/platformsdk` 内联副本手工同步（先平台后 service 纪律）。

**否决「在平台管理台全管」**：平台管理台账号是平台运营自己的（跨商户全局视角），商户无法自助；每个租户运营账号的加人/改密/授权都要平台运营介入，规模化后平台运营成瓶颈，且把商户业务运营塞回平台违背两仓分工。平台管理台保留为平台运营兜底入口（同一事实源，双入口无漂移）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 平台管理台全管（零 service 工作量） | 见上：商户不能自治，违背平台=基础设施/商户自治分工 |
| 只接管理面，门户身份维持 app 用户模式分期切 | 两套门户成员模型并存，商户侧看到两种「成员」且语义冲突（平台已定调 tenant_users 是正解）；本期一并切换成本可控 |
| 门户成员管理直调平台 `/tenant-api`（不经 service） | 门户前端由本服务托管，直调需平台 CORS 登记 + 暴露平台地址给租户侧浏览器；经服务代理统一边界（对齐 B/C 端模式） |
| 本地保留角色绑定作缓存 | 平台已下发 perm_codes（自省聚合），本地缓存只会引入第二事实源与失效复杂度 |

## 验收标准

- [x] 平台 `go build/vet` + `sdk/` 单测（含新域 contract 对账测试）通过；service `go build/vet/test`（含 tenantmember 守卫单测：最后管理员 5 分支/本人/越界定位）通过；双前端 `vue-tsc -b`+build、eslint 0 error
- [x] 冒烟（本地联调环境）：开放面目录（27 码）/角色创建（8 权限点）/用户创建/409 重名/租户过滤；门户登录→profile（perm_codes+本地租户视图）→成员列表/角色选项→门户内建成员→自改角色 400→自删 400→设备列表（真实数据 tid 过滤）→无角色用户 403（member 与 device 双端点）；登录频控触发并按窗口恢复
- [x] 冒烟数据清理（平台软删 2 用户+1 角色，tenant 71 复归 0 存量）；商户 scopes 已补 `tenant-user:*`/`tenant-role:*`（开发库=部署前置动作）
- [x] `tenant_user_roles` DropTable 幂等登记；`/app-api/v1` 仅剩 C 端探针端点
- [x] 修复冒烟发现的 `/members/roles` Go 默认键名 bug（补 RoleVO json tag）

## 风险与后果

- **门户切换是破坏性变更**：存量门户会话（app token）失效；存量「app 用户+本地 tenant_admin」门户管理员需在管理页手工重建为 tenant_users（平台三表各自唯一索引，撞名不冲突）。开发环境无存量，生产上线需公告。
- **开放面 tenant-user 域首用**：409 语义靠平台文案匹配（`mapTenantUserErr`/`mapTenantRoleErr`，与 mapAppUserErr 同脆弱点），平台文案变更需同步。
- **deploy 前置**：商户须配 `tenant-user:*`/`tenant-role:*` scopes（12 项），否则管理页 403；平台先行部署（含 SDK 新端点 `/open-api/v1/tenant-user-perms`）。
- **门户登录频控双计**：服务侧 LoginIPGuard（客户端 IP）+ 平台侧（服务出口 IP）同款防护叠加，正常流量无感、爆破场景更严。
- **CORS 收敛**：门户登录改走服务代理后，不再需要平台为门户前端登记 CORS（B 端同收益）；`web/src/api/platform.ts` 已删除。
- **最后管理员守卫的边界**：商户管理面（开放面直调）无此守卫——商户管理员可让租户失管后自行恢复，属设计内；门户层守卫主要防缓存窗口竞态与误操作。

## 交叉链接

- 取代：[2026-10-08-tenant-user-system.md](2026-10-08-tenant-user-system.md)（本仓上午版：app 用户+本地双角色门户——身份模型已被本文取代，登录/守卫/设备过滤骨架延续）
- 平台侧：`embodied-platform/aiDoc/notes/implemented/architecture/2026-10-08-tenant-user-system.md`（第四套身份三面落地 + SDK 补齐追注）
- 关键代码：`internal/biz/tenantuser/gateway.go`、`internal/data/platform/tenantuser_gateway.go`、`internal/data/platform/tenantidentity.go`、`internal/biz/auth/tenantauth.go`、`internal/server/middleware/middleware.go:TenantAuth/RequireTenantPerm`、`internal/server/handler_tenantuser.go`、`internal/server/handler_tenantapi.go`、`internal/service/tenantmember/`、`web/src/views/tenant/TenantUsers.vue`、`web/src/views/tenant/TenantRoles.vue`、`web/src/api/tenant.ts`、`web/src/stores/tenantUser.ts`
