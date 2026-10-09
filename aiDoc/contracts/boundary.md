<!-- last-updated: 2026-10-09 -->
# 契约层（boundary）

> 本项目为 mixed：本文件维护三条契约边界——**web 前端 ↔ 本服务 API**、**本服务 ↔ 平台开放面（服务端 HMAC）**、**本服务 ↔ 平台认证公开 API（管理端登录代理）**。契约结构来自真实代码。

## 责任边界

| 职责 | 归属 |
|---|---|
| 参数校验（binding）、业务规则、鉴权（RBAC/准入）、持久化、平台开放面代理 | 本服务（后端） |
| 表单即时反馈、展示格式化、路由/菜单按权限渲染 | web 前端 |
| 账号密码校验、token 签发/刷新/吊销、验证码 | **平台（embodied-platform，唯一身份源）**——管理端登录经本服务后端代理转发（密码过内存不落盘），本服务只做代理 + token 自省 + 登录日志 |
| 文件字节存储 | 平台 storage-gateway（本服务只存元数据，下载 302 到预签名 URL） |

## web-api 变体：HTTP 接口契约（前端 ↔ 本服务）

### 统一响应结构

```json
{ "code": 0, "msg": "ok", "data": { } }
```

| 字段 | 类型 | 说明 |
|---|---|---|
| code | int | 0 成功；1 业务错误；401 未认证；403 无权限 |
| msg | string | 成功固定 `ok`；失败为按请求 Accept-Language 本地化的文案 |
| data | any | 业务数据；失败/无数据时省略（`omitempty`） |

定义位置：`pkg/response/response.go:Body` 与 `CodeOK/CodeErr/CodeUnauthorized/CodeForbidden`；本地化经 `FailI18n`（哨兵错误→i18n key 注册表 `internal/server/i18n_errors.go:errKeys`）。

### 统一分页结构

列表接口 data 固定为：

```json
{ "list": [], "page": { "page": 1, "page_size": 20, "total": 100 } }
```

定义位置：`internal/server/handler_user.go:listResult`、`pkg/pagination/page.go:Page`。参数由 `pageParams` 统一解析（`internal/server/router.go:566`），`page_size=0` 表示全量（运行时有上限夹取）。

### 字段命名规范

- 全链路 **snake_case**（后端 json tag → 前端 `web/src/api/types.ts`），两侧不做任何风格转换
- 前端提交对象统一深度去首尾空格：`web/src/api/request.ts`（`deepTrim`，密码/文件除外）

### 关键类型桥接

- 状态类字段用 `int`（1 启用 0 禁用，如 `status`）；可缺省的请求字段用 `*int`/指针 + `omitempty`（见 `internal/service/dict/service.go:TypeCreateRequest.Status`）
- 文件下载不回传字节：`GET /files/:id/raw` 302 到平台存储预签名 URL（`internal/server/router.go` files 组）

### 认证与请求头

- 本服务 API：`Authorization: Bearer <平台 access_token>`；语言 `Accept-Language`（响应 msg 与菜单名均本地化）
- 路由分档（`internal/server/router.go:registerRoutes`）：basic（PlatformAuth，本人数据）→ protected（PlatformAuth+RBAC，默认拒绝）；App 面 `/app-api/v1/*`（AppAuth+租户闸门）与租户门户 `/tenant-api/v1/*`（TenantAuth+本地租户闸门）各自独立分档（见下两节）

## web-api 变体：App 面 HTTP 契约（C 端 App ↔ 本服务 `/app-api/v1`）

面向 C 端 App（独立客户端，不在本仓库；本仓库前端不消费本面）：

- **认证**：Bearer 平台 **app-access token**（App 直连平台 `/app-auth` 登录取得，token 双用）+ 必带请求头 `X-Tenant-ID`（**平台租户 ID**，须 ∈ 用户归属集且本地已同步启用，否则 403）。AppAuth 自省平台 app profile + 租户闸门；per-uid 限流 120/min
- **端点**（试点，业务端点按需求挂入）：`GET /profile`（本人 + 可访问租户本地视图）、`PUT /profile/password`（持本人 token 代理平台改密）
- **响应信封/分页/snake_case** 与管理端一致（同一 `pkg/response`/`pkg/pagination`）
- 决策记录：`aiDoc/notes/implemented/architecture/2026-10-05-appauth-and-appuser-unification.md`

## web-api 变体：租户门户 HTTP 契约（租户门户前端 ↔ 本服务 `/tenant-api/v1`）

面向企业租户的门户端（`web/src/views/tenant-portal/`）与商户管理端完全隔离，身份为平台第四套身份 tenant_users（单租户绑定），2026-10-08 起：

- **认证**：Bearer **tenant-access token**；登录/刷新经本服务后端代理平台 `/tenant-api/v1/auth`（token 双用于平台与本系统，本侧挂 LoginIPGuard + 登录限流 10 次/分/IP）。TenantAuth 以 token 自省平台 `/tenant-api/v1/profile` 得身份（tid 取自 token，单租户绑定，**不信任请求头，不携带 X-Tenant-ID**）+ 本地租户闸门（`tenants.platform_id` 已同步且启用，否则 403）；per-uid 限流 120/min
- **鉴权**：权限码由本地三表 JOIN（binds→roles→role_perms）在缓存装载时解析（变更 ≤60s 生效），`RequireTenantPerm` 精确匹配（成员自治 `member:*`、设备只读 `device:list`）；`GET /profile`、`PUT /profile/password` 为本人数据不校验权限码
- **响应信封/分页/snake_case** 与管理端一致（同一 `pkg/response`/`pkg/pagination`）
- **成员语义**：`/members/*` 经平台开放面 tid 锁定操作本租户 tenant_users；`DELETE /members/:id` = 移出租户（不删账号）；守卫：本人不可自操作、最后一名成员管理员不可失格（biz 层哨兵）
- 前端独立 axios 实例（`web/src/api/tenant.ts`，baseURL `/tenant-api/v1`，token 独立存储键 `tenant_access_token`/`tenant_refresh_token`，401 单飞刷新走本服务 `/tenant-api/v1/auth/refresh`）
- 决策记录：`aiDoc/notes/implemented/architecture/2026-10-08-tenant-user-system.md`

## 组件间契约：本服务 ↔ 平台开放面（服务端）

- 调用经 `internal/platformsdk/client.go:Client`（HMAC 签名 `signer.go`；直连 `/open-api/v1`，经网关 `/gw/open-api/v1`，签名按实际完整 path）；业务封装在 `internal/data/platform/`（设备/租户/型号同步、storage-gateway）
- 平台信封同为 `{code,msg,data}`，`code!=0` 报错；`cmd/server/main.go:checkPlatform` 启动时异步自检连通性
- `ListDevices` 支持可选 `tenant_id` 单租户过滤（平台租户 ID，⊆ 商户绑定租户集，越界 403）——租户端设备列表依赖此参数（2026-10-08 起，双侧已同步）
- 型号/物模型商户归属收敛（2026-10-08 起，跟随平台 `bdd1c9b9`）：`DeviceModel`/`TMNode` 带 `merchant_id`（0=通用 / >0=商户独立）；型号读=通用+本商户，创建 merchant_id 平台注入，**更新/删除仅本商户独立型号（通用/他商户统一 404，本服务 DeleteModel 不再幂等放行 404）**；物模型节点选择器同口径收敛；设备查询叠加型号可见性（他商户独立型号下设备列表过滤、子资源 404），设备注册 `model_id` 须对商户可见（否则 404）
- 数据映射域（2026-10-09 起，平台 `/open-api/v1/data-mappings*`，scope `mapping:*`）：15 端点全量代理到本服务 `/api/v1/data-mappings*`（biz `MappingGateway` ← data `MappingGatewayAdapter` 纯透传，无本地表）；同口径商户收敛（读=通用+本商户、写仅本商户独立 404、创建 merchant_id 平台注入）；版本管理=单草稿制 + 发布不可变 + 追加式回滚（`publish=true` 一步回退）；`GET /data-mappings/effective?model_id=` 无已发布版本返回 `data: null`（非错误）；平台信封错误经 `platformErr`（内聚 SDK 错误归一）按状态码透传
- 物模型写面（2026-10-09 起，平台 `/open-api/v1/thing-models*` 写端点，scope `thing-model:write`；读仍 `thing-model:read`）：13 端点代理到本服务 `/api/v1/thing-models*`（节点 CRUD + 版本 draft/publish/rollback/delete + `GET /:id/resolve?version_id=` 解析结果 + `GET /:id/inheritance-status` 继承滞后检测）；同口径商户收敛（通用节点只读，写仅本商户独立 404，创建 merchant_id 平台注入）；版本语义同数据映射（单草稿制、发布不可变、追加式回滚 `publish=true` 一步回退）；`TMSchema` 为 map 结构（要素名为主键，含 properties/services/events 三段），本服务 `TMSchemaUpdateRequest.Schema` binding required、原样透传；错误经 `platformErr` 透传平台 msg
- 配置：`configs/config.yaml` 的 `platform` 节（`appKey/appSecret`、storage `baseURL/apiKeyID`）
- **契约以平台仓库为准**（`embodied-platform/internal/server/openapi.go`）；接口/签名/字段变更须两侧仓库分别留痕（项目集根 `../../AGENTS.md`）

## 组件间契约：本服务 ↔ 平台认证公开 API（登录代理）

- 管理端登录/刷新/登出/验证码经本服务后端代理平台（2026-10-08 起，替代原浏览器直调）：前端只调本服务 `POST /api/v1/auth/login`、`POST /auth/refresh`、`GET /auth/captcha`、`POST /auth/logout`；后端 `internal/data/platform/identity.go:IdentityClient` 以普通 HTTP 调平台主服务同名公开端点（`/api/v1/auth/*`，非开放面、无 HMAC）
- 设计动机与备选见决策记录 `aiDoc/notes/implemented/architecture/2026-10-08-admin-login-backend-proxy.md`：服务端由此记录登录日志（成功+失败，login_logs 表）并恢复 LoginIPGuard 临时封禁与登录 IP 限流（10 次/分/IP）
- token 双用语义不变（平台签发，本服务自省）；平台 4xx 的 msg 原样透传给前端展示；平台不可达 → 503
- 租户门户登录/刷新同构代理（2026-10-08 起）：前端只调本服务 `POST /tenant-api/v1/auth/login`、`/auth/refresh`，后端经 `IdentityClient` 调平台 `/tenant-api/v1/auth/*`；本仓库前端已无浏览器直调平台认证面（`web/src/api/platform.ts` 已移除）。C 端 App 直联平台 `/app-auth`（独立客户端，不在本仓库），其 Origin 的 CORS 白名单属平台侧部署事项

## 变更规则

1. 契约字段的新增/改名/删除属于接口改动，必须同步契约两侧（后端 DTO ↔ 前端 `types.ts`；平台开放面两侧仓库）
2. 契约变更必须跑 typecheck + 契约级测试（后端 `make test` 聚焦包；前端 `npm run build` 含 vue-tsc）
3. 破坏性变更必须先写决策记录（见 [../notes/README.md](../notes/README.md)），并说明迁移与回滚方式

## 完成前检查清单

- [ ] 两侧字段名、类型、可空性一致（snake_case，无风格转换）
- [ ] 类型桥接在约定位置完成，未散落各处
- [ ] 错误响应对消费方可预期（信封结构、i18n key 已注册）
- [ ] 相关测试已覆盖契约变更点
