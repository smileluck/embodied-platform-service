<!-- last-updated: 2026-10-08 -->
# 租户用户体系：开放租户端给企业租户使用

> 路径：`aiDoc/notes/implemented/architecture/2026-10-08-tenant-user-system.md`

## 问题

企业租户（商户的客户组织）此前没有任何自助入口：本服务只有商户管理端（B 端，平台商户成员 + 本地 RBAC），租户（tenants）仅是设备归属分组与应用用户归属目标。需要一套「租户用户体系」让租户侧人员登录租户端，自助查看本租户设备、管理本租户成员、维护本人资料——且不得破坏「平台是唯一身份源」铁律。

## 提案 / 决策

**身份复用平台应用用户（AppUser），本服务只补「租户级授权层」**：

1. **租户端用户 = 平台 AppUser**：登录由租户端前端直调平台 `POST /api/v1/app-auth/login`（token 双用于平台与本服务 `/app-api/v1`），平台零改动即获得完整身份链（账密/禁用即时生效/token 刷新）。AppAuth 中间件（自省 + `X-Tenant-ID` 租户闸门）原样复用。
2. **本地授权层（新上下文 `internal/biz/tenantmember/`）**：新表 `tenant_user_roles(app_user_id, tenant_platform_id, role)`，内置双角色 `tenant_admin`/`member`（**不建角色表**，绑定缺失视为 member）。成员资料经 `appuser.Gateway` 实时消费平台开放面，本地不落成员数据面——与 B 端 admission 投影同一哲学：身份在平台，授权在本地。
3. **鉴权分档**：读面（设备列表/详情/影子/遥测、profile）「归属即准入」不挂角色校验；成员管理面挂 `middleware.TenantAdmin`（默认拒绝，仅 tenant_admin 放行）。绑定表主键双列点查，v1 不加缓存（per-uid 限流 120/min 兜底）。
4. **成员语义**：「移除成员」= `UpdateAppUser` tenant_ids **差集更新**（保留本商户其他租户归属与他商户归属），不删账号；硬删仅存在于 B 端管理面。
5. **守卫**：最后一个 tenant_admin 不可移除/降级/禁用（防租户失管）；本人不可移除/降级/禁用自己。
6. **登录自举**：平台 `/app-auth/login` 响应携带 `user.tenant_ids`，前端据此在调本服务 `/profile`（需 X-Tenant-ID）前确定当前租户——解决「租户集合未知 vs profile 需租户头」的先后依赖。
7. **前端形态**：同应用新增 `/tenant-portal/*` 路由树 + 独立登录页（前缀避开管理端「租户中心」菜单的 `/tenant/*`，2026-10-08 路由前缀冲突修复后定稿），token 独立存储键（`tenant_access_token`/`tenant_refresh_token`），独立 axios 实例（baseURL `/app-api/v1`，自动带 Bearer + X-Tenant-ID，401 单飞刷新）；静态菜单按 role 过滤（无后端菜单面）。
8. **平台侧 enabler（跨仓库）**：开放面 `ListDevices` 增加可选 `tenant_id` 过滤（⊆ 商户绑定租户集，越界 403，与 `AppUserFilter.TenantID` 同语义）——否则租户端设备列表只能本地过滤导致分页失真。`../embodied-platform` 与本仓 `internal/platformsdk` 已双侧同步。
9. **B 端管理面补充**：`/app-users` 创建时可指定 `tenant_roles`、`PUT /:id/tenant-role` 单独设置、列表带角色标注——租户首管理员的来源。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 平台新增「租户成员」账号类型与开放面 | 与 AppUser 模型重复，跨仓库大改；仅在需要应用用户与租户端用户严格隔离时才值得 |
| 复用 B 端商户成员 + 租户维度本地 RBAC | 平台 MerchantUser 无租户维度；B 端 protected 组全部端点需逐个加租户过滤，泄露整个管理面，改动面与风险远大于收益 |
| 本地自建租户用户表（账密本地） | 破坏「token 只由平台签发」铁律，重演 2026-10-05 已删除的本地登录面问题 |
| 租户级自定义角色（复用 role/permission 表） | v1 无此需求；内置双角色覆盖「管理员管成员、成员只读」主场景，后续可演进 |
| 角色绑定加二级缓存（同 B 端 RBAC） | 绑定查询是无 JOIN 的主键点查、成本恒定；per-uid 限流已兜住频率，缓存的一致性代价不划算 |
| 成员列表惰性清理孤儿绑定（按页 DeleteNotIn） | 分页视角无法区分「他页成员」与「已移除成员」，按页清理会误删；孤儿绑定在 AppAuth 闸门处天然失效（用户已不在 TenantIDs 即 403），属卫生问题——移除路径精确清理 + 未来的低频全量对账即可 |

## 验收标准

- [x] 租户端登录→选租户自举→设备列表（仅本租户）/详情/影子/遥测端点就绪（handler 层强制 X-Tenant-ID 过滤与归属校验）
- [x] tenant_admin 可增删成员/启停/重置密码/任命角色（`/app-api/v1/members/*`，TenantAdmin 鉴权）；member 被 403
- [x] 最后一个 tenant_admin 不可移除/降级/禁用；本人不可自我移除/降级/禁用（单测覆盖）
- [x] 跨租户操作被本地拒绝（成员定位按租户过滤；设备越权 404 语义不泄露存在性）
- [x] B 端创建应用用户时可设置租户端角色 + 行内调整
- [x] `go build ./...`、`go vet`、`go test ./internal/biz/tenantmember/... ./internal/server/middleware/...` 通过；`web` `vue-tsc -b` + `npm run build` + `npm run lint`（0 错误）通过；平台侧 `go test ./internal/service/openapi/` 通过
- [ ] 端到端冒烟（登录→租户切换→设备/成员/个人中心全链路）依赖平台联调环境，**未执行**

## 风险与后果

- **平台开放面无机器可读子码**：409 语义靠文案匹配（`mapAppUserErr` 既有脆弱点），租户端成员操作沿用该映射，平台文案变更需同步。
- **SetAppUserStatus/ResetPassword 是平台账号级操作**：租户管理员禁用/重置的成员若同时归属他租户/他商户，影响是全局的（B 端管理面同样如此）；文档已注明，v1 接受。
- **孤儿角色绑定**：B 端管理面直接改 tenant_ids 移出租户时，本地绑定残留（无精准清理点）；闸门保证其无安全影响，卫生清理留待后续低频对账任务（可选）。
- **租户端会话无服务端吊销**：平台 `/app-auth` 无 logout 端点，租户端登出仅本地清态，会话靠平台侧过期；平台侧如后续加 logout 应同步接入。
- **deploy 注意**：租户端前端 Origin 必须登记进平台 CORS 白名单（同 B 端 web 直调登录的模式）。
- **指令下发未开放**：`/app-api/v1` 设备面 v1 只读；开放前需单独决策（审计/限流/优先级策略）。

## 交叉链接

- 前置决策：[2026-10-05-appauth-and-appuser-unification.md](2026-10-05-appauth-and-appuser-unification.md)（AppAuth 骨架与「归属即准入」）、[2026-10-05-tenant-reconcile-and-gate-status.md](2026-10-05-tenant-reconcile-and-gate-status.md)（租户闸门启停校验）
- 变更计划：`aiDoc/plans/completed/2026-10-08-tenant-portal-user-system.md`
- 业务需求：`aiDoc/memory/business/2026-10-08-租户用户体系.md`
- 关键代码：`internal/biz/tenantmember/`、`internal/server/handler_tenantapi.go`、`internal/server/middleware/middleware.go:TenantAdmin`、`web/src/api/tenant.ts`、`web/src/stores/tenantUser.ts`、`web/src/views/tenant-portal/`
- 平台侧配套：`embodied-platform/internal/service/openapi/device.go`（ListDevices tenant_id 过滤）+ `sdk/client.go`（同口径变更已手工同步本仓 `internal/platformsdk`）

## 未涵盖（显式不做）

- `api/auth/v1/auth.proto` 与 `api/admin/v1/admin.proto` 的既有漂移（Login/Logout/AppAuthService/OnlineSessionService 等历史定义）——先于本次变更存在，另行清理。
- 租户端菜单/权限点的后端化（当前静态按 role 过滤足够两档角色；自定义角色时再引入）。
- `migrations/` 中残留的 `app_user_tenants` 索引引用（postgres/sqlite 参考脚本的既有漂移）。
