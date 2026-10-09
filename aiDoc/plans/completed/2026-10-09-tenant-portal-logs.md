<!-- last-updated: 2026-10-09 -->
# 租户门户登录/操作日志（独立两表 + 门户查询页 + 管理端可查）

## 目标

租户门户（/tenant-api/v1）的登录尝试与写操作落本地审计日志：独立两表 `tenant_login_logs`/`tenant_operation_logs`（均带 `tenant_id` 平台租户 ID）；租户管理员在门户「日志」页查本租户记录（新权限码 `log:list`，tid 强制取自 token）；商户管理端在日志管理下查/清各租户记录；保留期自动清理与手动清空跟随现有 log 机制（`log.retentionDays`）。

## 非目标

- C 端 `/app-api/v1` 日志（无写操作、消费方不在本仓库）
- 门户侧日志清空/导出入口；管理端租户日志导出
- 平台侧审计任何改动；登录限流/LoginIPGuard 策略变更

## 假设

- 门户登录成功后可用新 access token 同步自省 `TenantProfile` 补 tid/username（登录低频，一次额外平台调用可接受；自省失败 tid=0 仍落库）
- 新表名不撞 migrateLegacy 清理名单（已核对：名单仅 app_user_tenants/app_users/tenant_user_roles）

## 影响面

- 后端：`internal/biz/log`、`internal/data/{model,log,data.go}`、`internal/server/{handler_tenantapi,handler_log,router}.go`、`internal/server/middleware/oplog.go`、`internal/biz/tenantuser/permcatalog.go`、`internal/service/log`、`migrations/` 三方言
- 前端：门户（`api/tenant.ts`、`views/tenant-portal/Logs.vue`、`router/index.ts`、`layout/TenantLayout.vue`、i18n）+ 管理端（`api/{types,index}.ts`、`views/log/TenantLogs.vue`、`router/dynamic.ts`、i18n）
- 契约：`/tenant-api/v1/logs/*`（新）、`/api/v1/tenant-{login,operation}-logs`（新）

## 验收标准

- [ ] 门户登录成功/失败均落 `tenant_login_logs`（成功行 tid 非 0）
- [ ] 门户写操作落 `tenant_operation_logs`（tid 正确、password 类 body 全脱敏）
- [ ] `GET /tenant-api/v1/logs/login|operation`：`log:list` 放行且只见本租户，无权限码 403
- [ ] 管理端 4 端点受 RBAC 控制（种子权限点），`tenant_id` 筛选生效
- [ ] 定时保留期清理覆盖 4 张日志表
- [ ] 门户 Logs 页按 `log:list` 显隐；管理端 TenantLogs 页按种子菜单/权限渲染
- [ ] `go build ./... && make wire`、聚焦 go test、`npm run build` 全绿

## 工作项

| # | 工作项 | owner | 依赖 | 建议写范围 | 验证命令 | 状态 |
|---|---|---|---|---|---|---|
| 1 | biz/log 实体/Query/Repo 接口/Usecase 扩展 | AI | — | `internal/biz/log/` | `go build ./internal/biz/log/` | 已完成 |
| 2 | PO + 仓储队列/查询/清理 + AutoMigrate | AI | 1 | `internal/data/model/model.go`、`internal/data/log/repo.go`、`internal/data/data.go` | `go build ./internal/data/...` | 已完成 |
| 3 | 三方言 DDL | AI | 1 | `migrations/` | 目视对齐 sqlite.sql:243 风格 | 已完成 |
| 4 | TenantOpLog 中间件 + 动作名/脱敏扩展 | AI | 1 | `internal/server/middleware/oplog.go` | `go test ./internal/server/middleware/` | 已完成 |
| 5 | 门户登录落日志 + `/logs/*` 查询端点 | AI | 1,2,4,6 | `internal/server/handler_tenantapi.go`、`router.go` | `go build ./internal/server/` | 已完成 |
| 6 | 权限码 `log:list` 注册 | AI | — | `internal/biz/tenantuser/permcatalog.go` | `go build ./internal/biz/tenantuser/` | 已完成 |
| 7 | 管理端 4 端点 + actionNames + 种子菜单/权限 | AI | 1,2 | `internal/server/handler_log.go`、`router.go`、`internal/data/data.go` | `go build ./internal/server/` | 已完成 |
| 8 | service/log VO 与透传 | AI | 1 | `internal/service/log/service.go` | `go build ./internal/service/log/` | 已完成 |
| 9 | 前端门户 Logs 页 + 路由/菜单/i18n | AI | 5,6 | `web/src/{api/tenant.ts,views/tenant-portal,router,layout,locales}` | `cd web && npm run build` | 已完成 |
| 10 | 前端管理端 TenantLogs 页 + api/viewModules/i18n | AI | 7 | `web/src/{api,views/log,router,locales}` | `cd web && npm run build` | 已完成 |
| 11 | 文档留痕 + 决策记录 + 业务记忆 | AI | 全部 | `aiDoc/` | `check_sync.py` | 已完成 |

## 验证命令

```bash
go build ./... && make wire
go test ./internal/server/... ./internal/biz/... ./internal/biz/job/...
cd web && npm run build
# 冒烟：平台 :27080 + 本服务 :28180，门户登录/写操作/两则日志查询、管理端查询清空
```

## 回滚 / 迁移

纯新增：回退代码 + `DROP TABLE tenant_login_logs, tenant_operation_logs`（无存量数据迁移）；种子菜单/权限随种子器幂等，回滚不残留运行影响。

## 当前状态

2026-10-09 计划创建，实施开始。

## 交付摘要（完成时填写）

2026-10-09 交付，与计划基本一致：

- 后端：biz/log 扩展两实体 + Repo 6 方法；data 两 PO + 队列/查询/清理覆盖 + AutoMigrate；三方言 DDL；TenantOpLog 中间件（含 maskFullPaths 整体脱敏集合）+ 2 个聚焦测试；门户登录落日志（成功自省补 tid）；`log:list` 权限码入 permcatalog；门户 `GET /logs/login|operation`；管理端 4 端点 + 种子（menu:tenantLog + 4 权限点，管理员角色自动获得）；service/log VO 透传。`make wire` 零 diff。
- 前端：门户 Logs.vue（双 tab 只读 + 保留期说明 + 详情弹窗）、管理端 TenantLogs.vue（租户下拉 platform_id 口径 + 清空按权限点渲染）、路由 meta.perm 校验扩展、菜单按 hasPerm 显隐、i18n 双语（含 tenantRole log 组权限码名）。
- 冒烟（平台 27080 + 本服务 28180 + 前端 28170 真实入口）：登录成功/失败落表（tid=71/tid=0）；改密整体脱敏、成员新增字段级打码、动作名正确；无 log:list 403、无 token 401、有权限 200 且租户隔离；门户页面浏览器实测双 tab 渲染。冒烟数据已清理。
- 偏离计划处：1) 冒烟发现并修复 n-tabs 双 pane 同构组件复用导致筛选栏槽内容错绑（SearchCard/n-card 加互异 key，已记 lesson 2026-10-09-n-tabs-pane-slot-stale）；2) actionNames 管理端只补 2 条 DELETE（OpLog 不记 GET，GET 条目永不命中）。
- 遗留：管理端 TenantLogs 页与 4 端点的真实管理员会话验证未做（需管理员凭据），代码路径与门户侧共用已验证的 service/repo 层。
