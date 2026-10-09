<!-- last-updated: 2026-10-09 -->
# 架构与模块组织规则（architecture rules）

> 变体标注：本项目为 **mixed**（Go 后端 = 分层服务变体；Vue 3 前端 = 组件结构变体），按组件分节约束。所有规则引用实际代码中的类名和文件路径。

## 总原则

- 严格单向分层：`internal/server` → `internal/service` → `internal/biz` → `internal/data`；禁止跨层（handler 直查 DB）与反向依赖（biz import server/service）
- 每层只通过下层的公开接口交互；仓储接口定义在 biz（依赖倒置），实现在 data
- 跨上下文依赖只允许经**最小接口**在 wire 中绑定（见 `cmd/server/wire.go:wire.Bind`，如 `bizdevice.TenantRelinker` → `*biztenant.Usecase`）；禁止 biz 上下文之间直接 import 具体实现
- 依赖注入统一走 wire（Kratos 风格 ProviderSet：`bizSet` / `dataRepoSet` / `serviceSet`，见 `cmd/server/wire.go`），`wire_gen.go` 是生成物不手改

## 后端组件：分层服务

### 数据层（internal/data + internal/data/model）

- PO 定义与 PO↔领域转换统一放 `internal/data/model/model.go`（如 `DictTypePO` + `DictTypeToPO`/`DictTypeFromPO`）；PO 命名 `XxxPO`，表名 snake_case 复数
- 领域实体带 json tag（snake_case），PO 带gorm tag；两层字段变化必须同步转换函数
- 仓储实现模式（范本 `internal/data/dict/repo.go`）：
  - 构造 `NewRepo(d *data.Data) <ctx>.Repo`，全部操作 `r.data.DB.WithContext(ctx)`
  - GORM 错误→哨兵错误经 `mapXxxErr` 映射：`gorm.ErrRecordNotFound` → `ErrXxxNotFound`，唯一约束冲突经 `isUniqueViolation`（兼容 MySQL 1062 / PG 23505 / SQLite UNIQUE 文案）
  - 更新/删除后必须检查 `RowsAffected == 0` 并返回 NotFound（防幂等重复操作误报成功）
  - 模糊查询一律 `security.EscapeLike` + `ESCAPE '/'`（防 LIKE 通配注入，见 `internal/data/dict/repo.go:ListTypes`）
  - 分页用 `paginate` scope：`pageSize<=0` 表示全量
- 软删 + 唯一槽位释放：带唯一列的表删除时在事务内调 `data.ArchiveUniqueColumns`（`internal/data/archive_unique.go`），见 `internal/data/dict/repo.go:DeleteType`

### 契约层（internal/service）

- 每上下文一个 `internal/service/<ctx>/service.go`：请求 DTO + gin binding tag（范本 `internal/service/dict/service.go:TypeCreateRequest`）
- DTO 只做「绑定校验 → biz Input」转换，禁止业务判断；可缺省字段用指针（如 `Status *int` + `omitempty`）
- 更新语义约定：空字符串/零值 = 保持原值（由 biz usecase 解释）

### 业务逻辑层（internal/biz）

- 每上下文 `entity.go`（实体 + 哨兵错误 + Repo 接口）与 `usecase.go`（编排）；错误一律哨兵 `ErrXxx`（`errors.New`，中文文案），data 层负责映射
- 用例方法签名模式：`func (uc *Usecase) Xxx(ctx context.Context, ...) (..., error)`；分页列表返回 `(list, pagination.Page, error)`（`pkg/pagination`）
- 业务校验（查重、引用存在性）在 usecase 内完成，通过 repo 接口查询，不直接碰 GORM
- 进程内解耦用 `pkg/eventbus`，不在 biz 之间互相调用

### 入口层（internal/server）

- handler 命名 `internal/server/handler_<ctx>.go`，方法挂在 `HTTPServer` 上，小写不导出；只做参数提取→调 service→响应封装
- 公共助手：路径 id 解析 `idParam(c)`、分页参数 `pageParams(c)`（`internal/server/router.go:566`）、列表响应 `listResult{List, Page}`（`internal/server/handler_user.go:15`）
- 响应只经 `pkg/response`：成功 `response.OK(c, data)`；业务失败 `response.FailI18n`（哨兵错误先在 `internal/server/i18n_errors.go:errKeys` 注册 i18n key）；参数错误 `response.BadRequest` + `i18n.T(ctx, "common.invalid_params")`
- 绑定失败必须返回 `common.invalid_params`，不透传 validator 原文

### 注册机制

- 路由集中在 `internal/server/router.go:registerRoutes`：全局中间件链 `I18n/SecurityHeaders/CORS/XSSFilter/SQLInjectionGuard`（`IPBlacklist` 挂 `/api/v1` 组）；分组——公开（`/api/v1/auth/*`、`/tenant-api/v1/auth/*` 登录代理）、basic（`PlatformAuth + OpLog`，本人数据不做 RBAC）、protected（`PlatformAuth + OpLog + RBAC`，默认拒绝）、App 面 `/app-api/v1`（`AppAuth`+租户闸门）、租户门户 `/tenant-api/v1`（`TenantAuth`+本地租户闸门）
- RBAC 按 `UserID|Method|Path` 判定（`internal/server/middleware/middleware.go:RBAC`，二级缓存）；新路由进 protected 组即受权限点控制
- 新上下文必须三处同步：`wire.go` 对应 set 增 provider（跨上下文接口同 set 内 `wire.Bind`）→ `make wire` → `router.go` 注册

### 错误码分配

| code | 含义 | 定义 |
|---|---|---|
| 0 | 成功 | `pkg/response/response.go:CodeOK` |
| 1 | 通用业务错误 | `CodeErr` |
| 401 / 403 | 未认证 / 无权限 | `CodeUnauthorized` / `CodeForbidden` |

业务细分语义走 `msg`（i18n key），不新增 code 段；禁止各模块私造 code。

## 前端组件（web/）：组件结构

### 分层与依赖方向

```
views/*（页面）→ components/*（共享组件）→ stores/*（Pinia）→ api/*（请求层）→ api/types.ts（类型唯一真源）
```

- 页面组件放 `web/src/views/<域>/<Page>.vue`；跨页面复用组件提升到 `web/src/components/`
- 服务端数据类型只在 `web/src/api/types.ts` 声明（与后端 DTO snake_case 一一对应）
- 管理端请求走 `web/src/api/request.ts` 的 axios 实例（`baseURL: '/api/v1'`），含登录/刷新/登出（`/auth/*`，经本服务代理平台）；租户门户走 `web/src/api/tenant.ts` 独立实例（`baseURL: '/tenant-api/v1'`）——前端**不得**直调平台

### 状态与路由

- 全局状态仅 `web/src/stores/{user,settings,theme}.ts`；页面局部状态留在组件内，禁止为单页面建 store
- 动态路由：登录后按菜单生成，新页面必须在 `web/src/router/dynamic.ts:viewModules` 登记 `menu:<code>` → 组件映射
- 权限判断用 `useUserStore().has(code)`（`'all'` 为超管通配，语义与后端 RBAC `*/*` 一致）

### 样式与主题

- 主题三处同步：Naive UI 主题（`stores/theme.ts`）、全局样式（`web/src/styles/`）、图表配色；签名元素 sx-led/sx-plate 复用既有样式类，不另起炉灶
- 文案一律走 vue-i18n（`web/src/locales/zh-CN` 与 `en-US` 同步新增 key），组件内禁止硬编码双语文案
