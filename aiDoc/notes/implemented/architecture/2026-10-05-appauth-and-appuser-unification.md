<!-- last-updated: 2026-10-05 -->
# AppAuth 授权面 + 应用用户体系统一到平台

## 问题

本地应用用户体系与平台逐字复制、零互通（各自 app_users/bcrypt/JWT 密钥，且本地表无平台关联字段），App 直调本系统的需求确认后必须收敛。管理端 B 端已是「平台唯一身份源 + 本地投影」，应用用户（C 端）是最后一处重复人群。

## 提案 / 决策

**平台为应用用户唯一事实源；本系统对 C 端不碰账密、不签发、不换签、无本地投影/RBAC（归属即准入）。** 四个机制：

1. **AppAuth 中间件**（`internal/server/middleware/middleware.go`）：Bearer app-access token → token 哈希两级缓存（`pat:` 前缀，30s/60s，与 B 端 pid: 同权衡）→ 自省平台 `GET /api/v1/app-auth/profile` → 租户闸门（`X-Tenant-ID`=平台租户 ID，须∈用户归属集 ∩ 本地 `tenants.platform_id`；缺头 400，两种不可达同文案 403 不泄露细节）；平台不可达 fail-closed **503**（`response.ServiceUnavailable` 新助手；B 端 PlatformAuth 维持既有 500 行为不动）。
2. **App 面路由组** `/app-api/v1`（AppAuth + per-uid 限流 120/min，`ratelimit` 新增 `SubjectFunc`）：试点端点 `GET /profile` 返回用户 + 可访问租户的本地视图（platform_id↔local_id 映射）；业务端点按需求逐个挂入。
3. **管理面经开放面实时消费**：`internal/biz/appuser` 收敛为网关端口（哨兵+Gateway 接口+视图类型），`internal/data/platform/appuser_gateway.go` 实现（404→不存在、403→租户越界、409 按平台文案区分重名/跨商户删除）；`service/appuser` 管理方法走网关，VO 形状不变（tenant_names 由本地租户按 platform_id 映射补齐），前端仅租户下拉切 platform_id。
4. **本地登录面删除**（tag `pre-appuser-removal`）：/app-auth、AppJWT、data/appuser、实体/用例、PO/AutoMigrate/三方言 DDL 全删；`migrateAndSeed` 幂等 DropTable 清存量；`jwt.secret` 保留（agent/notify/mcp cryptoKey 派生源，配置注释已更新）；租户删除的 app_user 守卫职责移交平台（ErrTenantInUse）。

tenant_ids 全链路唯一口径为**平台租户 ID**（AppAuth 请求头、开放面、前端下拉/表单）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| service 做登录代理后本地换签 token | 重新造出第三套签发体系，破坏「token 只由平台签发」铁律 |
| C 端本地投影落库（类比 B 端 platform_users） | B 端落库是因为本地角色/准入管理；C 端闸门纯派生（归属即准入），落库只白养同步与孤儿回收 |
| 删除放行跨商户归属的用户 | 单商户无权处置他商户运营的用户；平台侧 409 拒删、归属商户各自先解除 |
| 平台 app token 加会话/黑名单做即时踢线 | 平台 AppJWT 每请求查库已保证平台侧即时生效；本系统仅余 30-60s 缓存窗口（冒烟实测：禁用后平台立即 401、service 窗口内 200、窗口后 401），未达需引入会话的痛点 |

## 验收标准

- [x] AppAuth 闸门矩阵单测：无/坏 token 401、平台不可达 503、缺头 400、租户不在归属集/本地未同步 403、缓存命中、主体注入。
- [x] `GET /app-api/v1/profile` 返回用户 + 租户上下文 + 可访问租户（平台↔本地 ID 映射）。
- [x] 管理面五操作经开放面走通；本地 app_users 表已随登录面删除（幂等 DropTable）。
- [x] 前端 tenant_ids/tenant_id 全为平台租户 ID（下拉仅列已同步租户）。
- [x] 联调冒烟（平台 :27080 + 本服务 :28180）：开放面建租户/建用户（子集正例 200、越界 403、重名 409、越界过滤=空集）；App 平台登录→token→/app-api/v1/profile 正例 200、缺头 400、租户 403、坏 token 401；禁用传播窗口实测符合设计；冒烟数据已清理。
- [x] `make test`/`go vet`/`make wire`/前端 `npm run lint && npm run build` 全绿。

## 风险与后果

- `internal/platformsdk` 是平台 `../embodied-platform/sdk/` 内联副本，须手工同步（平台 contract_test 只防平台侧漂移；按先平台后 service 顺序同 commit 期对齐）。
- 网关 409 区分依赖平台错误文案匹配（`已存在`），平台侧文案变更需同步 `mapAppUserErr`。
- 禁用/改归属在本系统的生效延迟 ≤ 缓存 TTL（30-60s），已在决策中接受。
- service 仓 harness manifest 基线存在先于本次变更的漂移（AGENTS/aiDoc 多文件，用户侧改动），未在本次刷新——需用户确认后重建基线。

## 交叉链接

- 平台侧配套（开放面 app-user 域）：`../embodied-platform/aiDoc/notes/implemented/architecture/2026-10-05-openapi-appuser.md`
- 变更计划：[../../plans/completed/2026-10-05-appauth-and-appuser-unification.md](../../plans/completed/2026-10-05-appauth-and-appuser-unification.md)
- 项目集对接流（C 端授权）：项目集根 AGENTS.md「五条对接流」
