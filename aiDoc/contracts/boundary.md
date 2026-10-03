<!-- last-updated: 2026-10-03 -->
# 契约层（boundary）

> 本项目为 mixed：本文件维护三条契约边界——**web 前端 ↔ 本服务 API**、**本服务 ↔ 平台开放面（服务端 HMAC）**、**浏览器 ↔ 平台（认证直调）**。契约结构来自真实代码。

## 责任边界

| 职责 | 归属 |
|---|---|
| 参数校验（binding）、业务规则、鉴权（RBAC/准入）、持久化、平台开放面代理 | 本服务（后端） |
| 表单即时反馈、展示格式化、路由/菜单按权限渲染 | web 前端 |
| 账号密码、token 签发/刷新/吊销、验证码 | **平台（embodied-platform，唯一身份源）**——本服务与前端均不碰密码 |
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
- 路由分档（`internal/server/router.go:registerRoutes`）：公开 `/app-auth/*`（本地应用用户）→ basic（PlatformAuth，本人数据）→ protected（PlatformAuth+RBAC，默认拒绝）

## 组件间契约：本服务 ↔ 平台开放面（服务端）

- 调用经 `internal/platformsdk/client.go:Client`（HMAC 签名 `signer.go`；直连 `/open-api/v1`，经网关 `/gw/open-api/v1`，签名按实际完整 path）；业务封装在 `internal/data/platform/`（设备/租户/型号同步、storage-gateway）
- 平台信封同为 `{code,msg,data}`，`code!=0` 报错；`cmd/server/main.go:checkPlatform` 启动时异步自检连通性
- 配置：`configs/config.yaml` 的 `platform` 节（`appKey/appSecret`、storage `baseURL/apiKeyID`）
- **契约以平台仓库为准**（`embodied-platform/internal/server/openapi.go`）；接口/签名/字段变更须两侧仓库分别留痕（项目集根 `../../AGENTS.md`）

## 组件间契约：浏览器 ↔ 平台（认证直调）

- 登录/刷新/登出由浏览器直调平台（token 双用）：`web/src/api/platform.ts`（`PLATFORM_API` 来自 `VITE_PLATFORM_API`，默认 `http://localhost:27080`；refresh_token 放 body）
- 平台是唯一身份源：本服务后端只做 token 自省 + 本地准入；前端 401 刷新重放逻辑在 `web/src/api/request.ts`
- 生产部署必须把本前端 Origin 登记进平台 CORS 白名单（平台 `cors.allowedOrigins`）

## 变更规则

1. 契约字段的新增/改名/删除属于接口改动，必须同步契约两侧（后端 DTO ↔ 前端 `types.ts`；平台开放面两侧仓库）
2. 契约变更必须跑 typecheck + 契约级测试（后端 `make test` 聚焦包；前端 `npm run build` 含 vue-tsc）
3. 破坏性变更必须先写决策记录（见 [../notes/README.md](../notes/README.md)），并说明迁移与回滚方式

## 完成前检查清单

- [ ] 两侧字段名、类型、可空性一致（snake_case，无风格转换）
- [ ] 类型桥接在约定位置完成，未散落各处
- [ ] 错误响应对消费方可预期（信封结构、i18n key 已注册）
- [ ] 相关测试已覆盖契约变更点
