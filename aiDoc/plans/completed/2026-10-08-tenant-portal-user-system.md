<!-- last-updated: 2026-10-08 -->
# 变更计划：租户用户体系（开放租户端）

## 目标

企业租户可登录租户端（web 内 /tenant-portal/* 路由区）并完成：本租户设备查看、成员自助管理（tenant_admin）、个人中心。身份复用平台应用用户，本服务新增租户级授权层（内置双角色）。

## 非目标

- 指令下发（端点结构预留，后续单独决策）
- 租户级自定义角色体系
- 独立前端应用（web-tenant/）
- 平台侧新增账号类型
- api/*.proto 既有漂移清理

## 假设

- 平台开放面 AppUserFilter.TenantID 可支撑按租户列成员；Update 的 tenant_ids 全量替换语义可安全实现「移除出本租户」
- 平台开放面 ListDevices 需新增 tenant_id 过滤（enabler，跨仓库 ../embodied-platform）
- SDK Device 视图含 tenant_id（Phase 3 核验，缺则平台补）
- 孤儿角色绑定在 AppAuth 闸门处天然失效（用户已不在 TenantIDs 即 403），惰性清理足够

## 影响面

- 后端：新上下文 tenantmember（biz/data/service/handler）、model/migrations、middleware.TenantRBAC、router /app-api/v1、wire、i18n errKeys；B 端 app-users 角色扩展
- 平台：../embodied-platform 开放面 ListDevices 过滤参数 + contract_test；本仓 internal/platformsdk 手工同步
- 前端：web/src/api/tenant、stores/tenantUser、router（/tenant-portal 树 + 守卫分流）、views/tenant-portal/*、i18n
- 契约：/app-api/v1 新端点（boundary.md 新增租户面契约段）；平台开放面设备列表参数（两侧留痕）

## 验收标准

- [ ] 租户端登录→选租户→设备列表（仅本租户）/详情/影子/遥测可用
- [ ] tenant_admin 可增删成员/启停/重置密码/任命角色；member 被拒（403）
- [ ] 最后一个 tenant_admin 不可移除/降级/禁用
- [ ] 跨租户操作被本地拒绝（成员/设备两侧）
- [ ] B 端创建应用用户时可设置租户端角色
- [ ] go build/vet + 聚焦单测通过；web typecheck/build 通过；平台 contract_test 通过

## 工作项

| # | 工作项 | owner | 依赖 | 建议写范围 | 验证命令 | 状态 |
|---|---|---|---|---|---|---|
| 1 | 平台 ListDevices tenant_id 过滤 + contract_test + SDK 同步 | AI | - | ../embodied-platform 开放面 + internal/platformsdk | 平台侧 go test ./internal/service/openapi/... | 未开始 |
| 2 | tenantmember 后端上下文八步 | AI | 1(设备过滤独立，可并行) | biz/data/service/handler/middleware/router/wire/migrations/i18n | go build ./... && go test ./internal/biz/tenantmember/... | 未开始 |
| 3 | /app-api/v1 设备租户面端点 | AI | 1 | handler_tenantapi 设备四端点 + 归属校验 | go build ./... | 未开始 |
| 4 | 前端租户端 | AI | 2,3 | api/store/router/views/i18n | npm run typecheck && npm run build | 未开始 |
| 5 | B 端 app-users 角色设置 | AI | 2 | handler_appuser + service + AppUsers.vue | go build + typecheck | 未开始 |
| 6 | 文档收口：决策记录/boundary/system-map/lessons/check_sync | AI | 2-5 | aiDoc 对应文件 | check_sync.py | 未开始 |

## 验证命令

```bash
go build ./... && go vet ./...
go test ./internal/biz/tenantmember/... ./internal/data/tenantmember/...
cd web && npm run typecheck && npm run build
# 平台侧（../embodied-platform）：go test ./internal/service/openapi/...
```

## 回滚 / 迁移

- 新表 tenant_user_roles 由 AutoMigrate 幂等创建；回滚 = 删表 + 回退代码
- 平台侧 ListDevices 过滤参数为可选，向后兼容，回滚无数据影响
- 前端 /tenant-portal/* 路由区独立，回滚不影响管理端

## 当前状态

2026-10-08：全部工作项完成，验证通过（端到端冒烟除外，依赖平台联调环境）。移入 completed/。

## 交付摘要（完成时填写）

**已交付**（开发完成，E2E 冒烟待平台联调环境）：

- **平台侧**（../embodied-platform）：开放面 ListDevices 增加可选 tenant_id 过滤（服务层 + HTTP 层 + contract 范围内测试 `TestListDevices_TenantFilter`）；`sdk/client.go` 同口径更新；本仓 `internal/platformsdk` 手工同步。部署备注：租户端 Origin 需登记平台 CORS 白名单。
- **后端**：新上下文 `tenantmember`（entity/usecase/repo/service）；`tenant_user_roles` 表（AutoMigrate + 三方言 DDL）；`middleware.TenantAdmin`；`/app-api/v1` 新端点：members 全套 CRUD/角色/密码 + 设备四只读端点（列表强制租户过滤、单查/影子/遥测前置归属校验）+ `PUT /profile/password`（本人改密代理平台 `/app-auth/password`）+ profile 扩展 role 字段；`biz/appuser.Gateway` 复用无改动；i18n 哨兵与词条（zh/en）齐备。
- **B 端管理面**：`/app-users` 创建支持 `tenant_roles`、`PUT /:id/tenant-role`、列表 `tenant_roles` 标注（按租户批量补齐）。
- **前端租户端**：`/tenant-portal/*` 路由树 + 守卫分流；`api/tenant.ts`（独立 axios + X-Tenant-ID + 401 单飞刷新）；`stores/tenantUser.ts`（独立 token 键 + 登录租户自举）；TenantLayout（租户切换器 + 角色过滤菜单）；页面：TenantLogin/Devices/DeviceDetail/Members/Profile；`locales/{zh-CN,en-US}/tenantPortal.ts`；B 端 AppUsers.vue 角色列与设置弹窗。
- **文档**：决策记录 `notes/implemented/architecture/2026-10-08-tenant-user-system.md`；boundary.md 修正过时「公开 /app-auth/*」描述并新增租户面契约段与 ListDevices tenant_id 说明；system-map 更新模块对应与上下文计数；业务记忆 + lessons（wire Bind 惯用法）。

**与计划的偏差**：

- 计划文件按实际执行日 2026-10-08 命名（批准文案为 10-07）。
- 「成员列表惰性清理孤儿绑定」改为「不做」：分页视角无法安全识别孤儿，孤儿绑定在 AppAuth 闸门处天然失效，移除路径已精确清理（决策记录备选表有完整论证）。
- 个人中心比计划更完整：确认平台有 `PUT /app-auth/password`，本人改密代理可用（计划原为降级预案）。

**验证结果**：本仓 `go build ./...`、`go vet ./internal/...`、`go test ./internal/biz/tenantmember/... ./internal/server/middleware/...`（含新 10 个用例）全部通过；web `vue-tsc -b`、`npm run lint`（0 错误，既有 any 警告不变）、`npm run build` 通过；平台仓 `go build ./...`、`go test ./internal/service/openapi/ -count=1`、`sdk` 模块 build+test 通过。**未执行**：端到端冒烟（需平台 + 本服务 + 前端联调环境）。

**遗留事项**：

1. E2E 冒烟（平台环境就绪后）：租户管理员建号（B 端）→ 租户端登录 → 设备/成员/个人中心全链路。
2. 平台 `/app-auth` 无 logout 端点：租户端登出仅本地清态（决策记录已注明）。
3. 孤儿角色绑定的低频全量对账任务（可选，卫生性质）。
4. 指令下发开放（独立决策）。
