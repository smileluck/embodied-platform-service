<!-- last-updated: 2026-10-09 -->
# 租户门户登录/操作日志（独立两表 + 双侧查询面）

## 问题

租户门户（/tenant-api/v1，平台第四套身份 tenant_users）自 2026-10-08 上线后，登录代理与成员自治写操作（增删成员/重置密码/设角色/启停/本人改密）在本地无任何审计：登录日志只有管理端记，OpLog 只挂 /api/v1。门户成员操作经开放面用的是商户级 HMAC 凭证，平台审计只能看到「商户应用」维度——本服务是唯一知道具体操作者是哪个 tenant_user 的位置，出了「谁把谁删了」无据可查。

## 提案 / 决策

独立两张表 `tenant_login_logs` / `tenant_operation_logs`（均带 `tenant_id` 平台租户 ID 列并加索引），复用 biz/log 的异步队列/保留期清理机制，双侧查询面：

1. **记录面**：
   - 登录：`tenantPortalLogin` 成功/失败均异步落 `tenant_login_logs`；成功时用新 access token 同步自省一次 `TenantProfile` 补 tenant_id/username（登录低频，一次额外平台调用可接受；自省失败零值落库），失败行 tenant_id=0（此时身份未知，属正常形态）。
   - 操作：新中间件 `TenantOpLog`（与 OpLog 同构）挂在门户 authed 组（TenantAuth 之后、RequireTenantPerm 之前，权限拒绝的尝试也记录）；操作人取 `TenantAuthSubject`，tid 取 `TenantAuthTenant().PlatformID`——不信任任何请求参数。password 类路由（门户改密、成员重置密码）body 整体脱敏（`maskFullPaths` 集合，对齐管理端 `/api/v1/auth/password` 口径）。
2. **查询面**：
   - 门户：`GET /tenant-api/v1/logs/login|operation`（新权限码 `log:list`，加入 permcatalog 本地注册表 log 组；tid 强制 = token tid，只读无清空），前端门户新增「日志」页（n-tabs，按 `hasPerm('log:list')` 显隐）。
   - 管理端：`GET/DELETE /api/v1/tenant-login-logs|tenant-operation-logs`（protected + RBAC；新菜单 `menu:tenantLog` 挂日志管理下 + 4 个权限点种子；GET 支持 `tenant_id` 精确筛选），前端 `views/log/TenantLogs.vue`。
3. **清理**：跟随现有机制——`log.retentionDays` 定时清理覆盖 4 张日志表（biz/job CleanupExpired 同一 Repo），手动清空入口仅管理端（与现有日志同语义：清 cutoff 之前）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 复用现有两表 + channel/actor_type 列 | 身份域撞号（tenant_user 与管理端用户 ID/用户名无区分维度）；开放给租户管理员时「共享表按 channel 过滤」的授权边界不如「表即域」清晰；保留/清理策略无法独立 |
| 不记，靠平台审计 | 平台开放面调用只有商户维度，丢失具体操作者，安全审计刚需补不上 |
| 登录日志不落 tid（统一 0） | 门户按 tid 过滤时自己的成功登录全部不可见，页面失去意义；一次自省换 tid 值得 |

## 验收标准

- [x] 门户登录成功/失败均落表（成功行 tid=71 实测；失败行 tid=0 + 平台 msg）
- [x] 门户改密/新增成员/移除成员落表：tid 正确、动作名正确、改密 body 整体脱敏「（已脱敏）」、成员创建 password 字段级打码
- [x] `GET /tenant-api/v1/logs/*`：无 `log:list` 403、无 token 401、有权限 200 且只见本租户（tid=0 失败行对门户不可见）
- [x] 管理端种子：`menu:tenantLog`（parent menu:log）+ 4 权限点入库，管理员角色自动获得
- [x] 门户「日志」页浏览器实测渲染（双 tab、筛选、保留期说明、详情弹窗）；修复了 n-tabs 双 pane 同构组件复用导致的筛选栏错绑（见 lessons/2026-10-09-n-tabs-pane-slot-stale.md）
- [ ] 管理端 TenantLogs 页真实入口验证（需管理员凭据，留待用户确认）

## 风险与后果

- 纯新增，无存量迁移；回滚 = 回退代码 + DROP 两表
- 登录路径多一次平台自省调用（仅成功分支）；平台不可达时登录本就失败，无新增失败面
- 管理端清空为全租户口径（与现有日志清理同语义），按租户清理未提供（后续有需求再加）
- 门户无日志清空/导出；管理端租户日志无导出（与现有导出不对齐，待有需求再补）

## 交叉链接

- 计划：`aiDoc/plans/completed/2026-10-09-tenant-portal-logs.md`
- 身份背景：`2026-10-08-tenant-portal-tenantusers-migration.md`、`2026-10-09-tenant-rbac-localization.md`
- 管理端登录日志同构先例：`2026-10-08-admin-login-backend-proxy.md`
- 相关 lesson：`../../memory/lessons/2026-10-09-n-tabs-pane-slot-stale.md`
