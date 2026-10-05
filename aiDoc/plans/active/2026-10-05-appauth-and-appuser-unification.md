<!-- last-updated: 2026-10-05 -->
# AppAuth 授权面 + 应用用户管理面统一到平台

> 路径：`aiDoc/plans/active/2026-10-05-appauth-and-appuser-unification.md`

## 目标

- App 直连平台 `/api/v1/app-auth` 认证，同一 access token 直调本服务新路由组 `/app-api/v1`（token 双用，同 B 端模式）；本服务不碰 C 端账密、不签发/换签 C 端 token。
- 新增 `AppAuth` 中间件：token 哈希两级缓存（同 PlatformAuth 形态）→ 自省平台 `GET /api/v1/app-auth/profile` → 租户闸门（`X-Tenant-ID` 须为平台租户 ID，∈ 用户归属集 ∩ 本地租户 `PlatformID`）。
- 应用用户管理页（`/api/v1/app-users`）改经平台开放面（HMAC）实时消费，路径与前端交互不变。
- 删除本地 appuser 登录体系（本地无真实存量数据，直接删，无迁移；删除前打 tag `pre-appuser-removal`）。

## 非目标

- 不做 C 端本地投影落库、不做本地 C 端 RBAC（归属即准入）。
- 不给平台 app token 引入会话；接受 30-60s 自省缓存窗口作为禁用生效延迟。
- 本期不挂 App 业务端点，仅授权基础设施 + `GET /app-api/v1/profile` 试点端点。
- 不删管理页全链路（前端页面、`menu:appUser`/`appUser:*` 权限种子、代理 handler 与 Gateway 保留）。

## 假设

- 平台侧开放面 app-user 域先于本计划阶段 3 落地（平台仓 `aiDoc/plans/active/2026-10-05-openapi-appuser.md`）。
- 平台主服务与开放面同源（`platform.baseUrl` 复用，无新增地址配置）。
- `jwt.secret` 同时是 agent/notify/mcp cryptoKey 派生源——删 AppJWT 用途时该配置必须保留。

## 影响面

- 后端：`internal/server/middleware`（AppAuth）、`internal/server/router.go`（/app-api/v1 组）、`internal/data/platform`（app 自省客户端 + AppUserGateway）、`internal/biz/auth`（AppIdentitySource）、`internal/service/appuser`（重写为代理）、`internal/platformsdk`（app-user 方法，与平台 `sdk/` 手工同步）、`internal/biz/appuser`+`internal/data/appuser`+PO+AutoMigrate+migrations（删除）、`internal/data/tenant/repo.go` 租户删除守卫（移除 app_user_tenants 计数）、`internal/data/data.go` migrateLegacy（DropTable 幂等清理）、`cmd/server/wire*.go`。
- 前端：`web/src/api`（AppUsers 方法对齐代理层）、租户列表 VO 暴露 `platformId`（tenant_ids 口径统一为平台租户 ID）。
- i18n：本地 appuser 登录相关哨兵词条清理。

## 验收标准

- [ ] `AppAuth` 闸门矩阵单测：无 token/坏 token 401、平台不可达 503 fail-closed、缺 `X-Tenant-ID` 400、租户不在归属集/本地无此租户 403、缓存命中路径。
- [ ] `GET /app-api/v1/profile` 试点端点：返回 App 用户 + 可访问租户（本地 ID 映射 + 平台 ID）。
- [ ] 管理页列表/创建/更新/重置密码/删除走平台开放面，本地 `app_users` 表无写入；前端无感（路径不变）。
- [ ] 租户下拉与提交的 tenant_ids 均为平台租户 ID（列表 VO 暴露 platformId）。
- [ ] 本地 `/app-auth`、AppJWT、`data/appuser`、PO、DDL、守卫计数全部移除；`make wire` diff 干净；`migrateLegacy` 幂等 DropTable。
- [ ] `jwt.secret` 保留且 agent/notify/mcp 派生不受影响。
- [ ] 决策记录落档；变更计划补交付摘要移入 `completed/`。

## 工作项

| # | 工作项 | owner | 依赖 | 建议写范围 | 验证命令 | 状态 |
|---|---|---|---|---|---|---|
| 1 | AppAuth：自省客户端 + 中间件 + /app-api/v1 组 + 试点端点 + per-uid 限流 | service | 平台 /app-auth/profile（已存在） | `data/platform/identity.go` 同构新增、`middleware.go`、`router.go`、`ratelimit.go` | `go test ./internal/server/... ./internal/data/platform/` | 未开始 |
| 2 | platformsdk app-user 方法（与平台 sdk/ 手工同步） | service | 平台工作项 5 | `platformsdk/types.go`、`client.go` | `go test ./internal/platformsdk/` | 未开始 |
| 3 | AppUserGateway + service/appuser 重写为代理（路径不变） | service | 2 | `data/platform/`、`service/appuser/`、`server/handler_appuser.go` | `go test ./internal/...`（聚焦） | 未开始 |
| 4 | 前端：租户 VO 暴露 platformId、AppUsers API 对齐 | service | 3 | `web/src/api/`、租户 VO | `npm run lint && npm run build` | 未开始 |
| 5 | 删除本地 appuser 登录体系（先打 tag） | service | 3,4 验收通过 | 见影响面删除清单 | `make wire && make test && make lint` | 未开始 |
| 6 | 决策记录 + 收尾闸门（check_sync + lessons） | service | 5 | `aiDoc/notes/`、harness 脚本 | `python3 aiDoc/.../check_sync.py` | 未开始 |

## 验证命令

```bash
cd embodied-platform-service
make wire && make test && make lint
cd web && npm run lint && npm run build
# 联调冒烟（平台 :27080 + 本服务 :28180）：
# 1) HMAC 建租户+app user（子集/越权断言）2) 平台登录拿 token
# 3) /app-api/v1/profile 正例 + 坏 token/租户负例 + 停平台 503
# 4) 平台禁用用户 → 30-60s 内本服务拒绝 5) 管理页冒烟（本地表无写入）
```

## 回滚 / 迁移

- 阶段 1/2/3 各自独立 commit，可单独 revert；删除前打 tag `pre-appuser-removal`。
- 无数据迁移（本地无真实存量）；`migrateLegacy` DropTable 幂等可重放。

## 当前状态

- 2026-10-05 计划建立。执行顺序：平台先行（其仓独立计划），本仓阶段 1 可先行开发、联调依赖平台。

## 交付摘要（完成时填写）
