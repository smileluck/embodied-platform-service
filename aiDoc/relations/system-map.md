<!-- last-updated: 2026-10-09 -->
# 系统地图（system map）

> 系统架构与组件关系。结构描述必须与真实目录一致，禁止虚构目录。

## 根目录职责

| 目录 | 职责 | 归属组件 |
|---|---|---|
| `cmd/server/` | 入口 `main.go` + wire 注入（`wire.go` 构建、`wire_gen.go` 生成物） | 后端 |
| `internal/biz/` | 领域层：实体/哨兵错误/Repo 接口/用例（25 个限界上下文） | 后端 |
| `internal/data/` | 基础设施层：GORM 仓储、`model/` PO 定义、`platform/` 平台客户端 | 后端 |
| `internal/service/` | 应用服务层：DTO、binding 校验、biz Input 转换 | 后端 |
| `internal/server/` | HTTP 层：`router.go` 路由注册、`handler_*.go`、`middleware/` | 后端 |
| `internal/conf/` | 配置结构与加载（viper，env 可覆盖） | 后端 |
| `internal/platformsdk/` | 平台开放 API 客户端（HMAC 签名、信封解包） | 后端 |
| `pkg/` | 公共包：response/security/pagination/cache/i18n/logger/eventbus | 后端 |
| `api/` | proto 形式对外 API 描述（`admin/v1`、`auth/v1`） | 后端 |
| `migrations/` | mysql/postgres/sqlite 三方言 DDL | 后端 |
| `configs/` | 运行配置（`config.yaml`） | 后端 |
| `web/` | Vue 3 管理前端（`src/{views,stores,api,router,locales,styles,utils,components,layout}`） | 前端 |

## 后端分层关系

```
cmd/server/main.go（wire 装配）
  └─ internal/server/router.go:NewHTTPServer（中间件链 → 分组注册）
       └─ handler_<ctx>.go（参数解析/响应封装）
            └─ internal/service/<ctx>/service.go（DTO + binding → biz Input）
                 └─ internal/biz/<ctx>/usecase.go（业务编排，只依赖 Repo 接口）
                      └─ internal/data/<ctx>/repo.go（GORM 实现 + PO↔领域转换）
                           └─ internal/data/model/model.go（PO）+ data.Data（DB 工厂）
```

- 调用方向必须单向：server → service → biz → data；禁止跨层（handler 直查 DB）与反向（biz import server）
- 仓储接口定义在 biz（`internal/biz/<ctx>/entity.go:Repo`），实现在 data（`internal/data/<ctx>/repo.go`）——依赖倒置是本仓库的硬约束
- 各层约束详见 [../modules/architecture-rules.md](../modules/architecture-rules.md)

## 核心基础设施

| 位置 | 职责 |
|---|---|
| `internal/data/data.go:NewData` | 按配置创建 mysql/postgres/sqlite 连接、连接池、AutoMigrate 与种子数据 |
| `internal/data/archive_unique.go` | 软删后归档唯一列（释放唯一槽位允许同码重建） |
| `internal/data/softdelete.go` / `jwt.go` / `cache.go` / `redis.go` | 软删过滤、JWT 签发、缓存封装、Redis 工厂 |
| `internal/data/platform/` | 平台 open-api 客户端（设备/租户/型号同步）与 storage-gateway 客户端（文件） |
| `internal/platformsdk/` | HMAC 签名器与开放面请求底层（`client.go`、`signer.go`、`types.go`） |
| `internal/server/middleware/` | I18n/CORS/安全头/XSS/SQL 注入防护/IP 黑名单/PlatformAuth/AppJWT/RBAC/OpLog/限流 |
| `internal/server/i18n_errors.go` | 哨兵错误 → i18n key 注册表（`response.ErrKeyFunc` 消费） |
| `pkg/cache` | TwoLevel 二级缓存（L1 内存 + L2 Redis，singleflight 回源） |
| `pkg/eventbus` | 进程内事件总线（跨上下文解耦） |
| `pkg/logger` | zap 封装（`Init`/`Sync`，按配置分级） |

## 前端数据流

```
web/src/views/*（页面组件）
  ← web/src/stores/*（Pinia：user/settings/theme）
  ← web/src/api/index.ts（本服务 API，axios 实例 web/src/api/request.ts：/api/v1、token、401 刷新重放）
  ← web/src/api/tenant.ts（租户门户独立 axios 实例：/tenant-api/v1、tenant-access token、401 单飞刷新；登录/刷新经本服务代理平台，前端不直调平台）
web/src/router/dynamic.ts（菜单 code → viewModules 组件映射，登录后动态挂路由）
web/src/locales/{zh-CN,en-US}（i18n 语言包，后端按 Accept-Language 同步本地化）
```

- 开发态 Vite 把 `/api` 与 `/tenant-api` 都代理到 `localhost:28180`（`web/vite.config.ts`；租户门户 baseURL `/tenant-api/v1`）
- 前端不直调平台（认证全走本服务代理），无 `VITE_PLATFORM_API` 类平台地址配置

## 模块对应关系

后端限界上下文 ↔ 前端页面（菜单 code 见 `web/src/router/dynamic.ts:viewModules`）：

| 后端上下文 | 前端页面 |
|---|---|
| `biz/permission` + `biz/role` + 用户（平台同步） | `views/system/{Users,Roles,Menus}.vue` |
| `biz/dict` / `biz/sysconfig` / `biz/notice` / `biz/job` | `views/system/{Dicts,Configs,Notices,Jobs}.vue` |
| `biz/file`（平台存储代理） | `views/file/Files.vue` |
| `biz/blacklist` / `biz/monitor` / `biz/log`（含租户门户审计两表 tenant_login_logs/tenant_operation_logs，见 [../notes/implemented/architecture/2026-10-09-tenant-portal-logs.md](../notes/implemented/architecture/2026-10-09-tenant-portal-logs.md)） | `views/system/Blacklist.vue`、`views/system/ServerMonitor.vue`、`views/log/{OperationLogs,LoginLogs,TenantLogs}.vue`、`views/tenant-portal/Logs.vue` |
| `biz/tenant` / `biz/appuser` | `views/tenant/{Tenants,AppUsers}.vue` |
| `biz/tenantmember`（租户门户授权层：/tenant-api/v1 成员自治 + 设备只读，见 [../notes/implemented/architecture/2026-10-08-tenant-user-system.md](../notes/implemented/architecture/2026-10-08-tenant-user-system.md)） | `views/tenant-portal/*`（租户门户，/tenant-portal/* 路由树 + `layout/TenantLayout.vue`，API 走 `web/src/api/tenant.ts` → /tenant-api/v1） |
| `biz/device` / `biz/devmodel`（平台开放面；devmodel 含数据映射子域 `MappingGateway`/`MappingUsecase`） | `views/device/{Devices,DeviceModels}.vue`（物模型/数据映射页随 `plans/active/2026-10-09-thingmodel-datamapping-pages.md` 阶段 3 落地） |
| `biz/agent` / `biz/mcp` / `biz/skill` | `views/agent/*`、`views/mcp/Servers.vue`、`views/skill/Skills.vue` |
| `biz/notify` | `views/notify/{Channels,Rules,Records}.vue` |
| `biz/dashboard` / `biz/export` | `views/dashboard/Dashboard.vue`（导出复用各列表页按钮） |

## 配置文件

| 文件 | 用途 |
|---|---|
| `configs/config.yaml` | 运行配置：server/db/cache/log/platform（开放面 appKey/appSecret、storage-gateway）等 |
| `.env.example` | 前端/部署环境变量样例 |
| `Makefile` | 全部构建/开发/部署目标（Windows 兼容分支） |
| `.air.toml` / `.air.windows.toml` | 后端热重载配置（端口 28180） |
| `web/vite.config.ts` | 前端构建与 `/api` 代理（端口 28170） |
| `Dockerfile` / `docker-compose.yml` | app + MySQL + Redis 编排部署 |
| `migrations/*.sql` | 三方言 DDL（AutoMigrate 之外的存量结构） |
