<!-- last-updated: 2026-10-09 -->
# 数据映射代理链路（平台开放面 /data-mappings 全量透传）+ 物模型/数据映射权限种子

- 日期：2026-10-09
- 状态：implemented

## 问题

平台仓库 2026-10-09 上线开放面数据映射域 `/open-api/v1/data-mappings*`（15 端点，scope `mapping:*`；读=通用+本商户、写仅本商户独立 404、创建 merchant_id 服务端注入）。本服务需为「物模型只读页 + 数据映射管理页」（计划 `aiDoc/plans/completed/2026-10-09-thingmodel-datamapping-pages.md` 阶段 2）提供后端代理链路与菜单/按钮权限种子。另有存量隐患：`platformErr` 只识别 `*platform.Error`，devmodel 等 SDK 代理链路的平台信封错误（`*platformsdk.Error`）会落成 502 而非按状态透传（与 `data/devmodel/repo.go` DeleteModel 注释声明的「由 platformErr 映射」不符）。

## 提案 / 决策

1. **映射域全链路纯透传**（照 devmodel 型号域模式）：`internal/platformsdk` 镜像平台 SDK 类型与 15 方法 → `biz/devmodel/mapping.go`（镜像类型 + `MappingGateway` 接口 + `MappingUsecase` 纯透传，无本地表）→ `data/devmodel/mapping.go`（`MappingGatewayAdapter` 类型转换，404/409 不幂等放行）→ `service/devmodel/mapping.go`（DTO type alias 复用 biz 类型）→ `internal/server/devmodel.go` 15 handler + `router.go` `/data-mappings` 组（静态段 `/effective`、`/versions/:vid` 与 `/:id` 同层共存，gin 静态段优先）。商户收敛判断不落地本服务（平台保证，重复判断只会口径分叉）。
2. **`platformErr` 内聚 SDK 错误归一**：把 `normalizeSDKError` 上提到 `platformErr` 入口（幂等，device 域先归一再进的行为不变），修复 devmodel 链路 4xx 落 502 的隐患，映射域 handler 得以一律直调 `platformErr`。
3. **权限种子全端点覆盖优先于码数精简**（14 个 mapping:* 码 + 2 个 thingModel:* 码）：`permission.Match` 为 method + path glob（`*` 单段、`**` 跨段）、一码一行、RBAC 任一码命中即放行。据此 `mapping:view` 绑 `GET /api/v1/data-mappings/*`（单段 `*` 同时覆盖 `/:id` 详情与 `/effective` 生效查询）；版本/绑定/发布/回滚子资源各列权限点（防粗粒度点越级吞细粒度点，与 `systemButtonPerms` 注释既有规则一致）；`thingModel:list/version` 与既有 `model:tmPicker/model:tmVersion` 同 path 并存（任一匹配放行，两个页面各自挂码）。
4. **菜单种子**：`menu:thingModel`（Sort 3）、`menu:dataMapping`（Sort 4）挂 `menu:deviceCenter`；菜单名 i18n 词条 `menu.menu:thingModel` / `menu.menu:dataMapping` 补 zh/en。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 映射域并入既有 `GatewayAdapter`/`Usecase`/`Service`（不拆 MappingXxx） | 15 方法涌入会让型号域文件失焦；独立 `MappingGatewayAdapter`/`MappingUsecase`/`MappingService` 与 biz 接口一一对应，wire 绑定更清晰 |
| 权限码精简为 7 个（list/view/create/update/delete/publish/bind）一码多 path | `Permission` 一码仅一行 method+path，且 `*` 不跨段——精简码无法覆盖全部 15 个真实端点 path，未覆盖端点会 403；子资源单列是 `systemButtonPerms` 注释的既有规则 |
| `mapping:view` 拆 `mapping:effective` 单列一条 | `effective` 是单段，`/api/v1/data-mappings/*` 已覆盖；code 唯一约束下同码不能两行，单列只会多一个同语义码 |
| 不动 `platformErr`，映射域 handler 学 device 域先 `normalizeSDKError` | 存量 devmodel handler 已在直调 `platformErr`（502 隐患）；归一上提一处修复全部 SDK 代理链路，device 域双归一幂等无副作用 |

## 验收标准

- [x] `go build ./...` 通过；`go test ./internal/platformsdk/... ./internal/biz/devmodel/... ./internal/data/devmodel/... ./internal/service/devmodel/...` 全绿（含新增 mapping 适配器 404 透传 / effective nil,nil / 嵌套转换三测）
- [x] wire 重生成（`wire ./cmd/server`），`wire_gen.go` 保留 tenantuser 在途装配并接入 mapping 链路
- [x] 同 gin 版本路由形态冒烟：15 条路由注册无 panic（静态段/参数段同层共存）
- [ ] 重启后种子自愈：设备中心下出现 物模型/数据映射 菜单，mapping:*/thingModel:* 按钮绑定商户管理员角色（随阶段 4 设备可见性验证一并核查）

## 风险与后果

- `platformErr` 行为变化：SDK 代理链路（devmodel 型号/物模型 + 本次映射域）平台 4xx 由 502 改为按状态码透传 msg——属修复性收敛，与 device 域既有行为对齐
- 权限码数量膨胀（14 码）是有意取舍：换「每个真实端点都有 RBAC 覆盖 + 子资源粒度可独立授权」
- 前端阶段 3 依赖本链路：`/api/v1/data-mappings*` 契约 = 平台开放面字段原样（snake_case），`effective` 无已发布版本返回 `data: null`
- 回滚：纯增量 + 单点归一，`git revert` 即可；种子按 code 幂等，删除种子行不会清理已入库权限点（需另行加 obsoletePermCodes）

## 交叉链接

- 计划：`aiDoc/plans/completed/2026-10-09-thingmodel-datamapping-pages.md`（阶段 2）
- 平台侧真源：`embodied-platform/sdk/types.go`、`embodied-platform/sdk/client.go`（数据映射域段）、`embodied-platform/internal/server/openapi_mapping.go`
- 同模式先例：`aiDoc/notes/implemented/architecture/2026-10-08-model-merchant-scope-resync.md`
- 本仓库契约：`aiDoc/contracts/boundary.md`（组件间契约节）
