<!-- last-updated: 2026-10-09 -->
# 物模型写面代理链路（本服务镜像平台开放面写端点）

## 问题

平台开放面刚上线物模型写端点（节点 CRUD + 版本 draft/publish/rollback/delete + resolve + inheritance-status，scope `thing-model:write`）。本服务物模型此前只有只读代理（节点/版本列表），需扩展为完整管理链路，支撑物模型页从只读升级为商户可管理（见计划 `aiDoc/plans/completed/2026-10-09-thingmodel-manageable.md`）。

## 提案 / 决策

按数据映射域既有范式做纯透传代理链路，13 端点：

- `internal/platformsdk/`：`TMSchema` 族类型镜像平台（map 结构，要素名为主键，含 properties/services/events 三段），`TMNodeCreateRequest` 不收 `merchant_id`（平台注入），client 追加 11 方法（节点详情/写、草稿、发布、回滚、删版本、resolve、继承状态）。
- `internal/biz/devmodel/`：`Gateway` 接口追加 11 方法，`thingmodel.go` 新建请求/镜像类型（binding 口径逐字段对齐平台 `openapi_thingmodel.go`：layer oneof=base category model instance、code/name max=64、status oneof=1 2），Usecase 纯透传。
- `internal/data/devmodel/`：`GatewayAdapter` 追加 11 方法 + SDK↔biz 转换；DeleteTMNode 404 不透传幂等放行（同 DeleteModel 语义，平台对通用/他商户节点统一 404）。
- `internal/server/`：`tms` 组扩为 13 条路由；错误一律 `platformErr`（内聚 SDK 错误归一）透传平台 msg；`resolve` 的 `version_id=0` 表示不 pin 版本。
- 权限种子 `systemButtonPerms` 新增 9 码挂 `menu:thingModel`（view/nodeCreate/nodeUpdate/nodeDelete/draft/draftUpdate/publish/rollback/versionDelete）。
- 商户收敛口径与既有只读一致：读=通用+本商户，写仅本商户独立（平台统一 404），创建 merchant_id 平台注入。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| `thingModel:view` 用单段通配 `GET /api/v1/thing-models/*` 覆盖详情 | 单段 `*` 不跨段，盖不住 `/:id/resolve`、`/:id/inheritance-status`；按 `file:view` 先例改用 `**` |
| draft 与 draftUpdate 合一码 | 一码一 path 是种子表既有约定（draft=POST `/*/versions`，draftUpdate=PUT `/versions/*`，path 不同故拆两码） |
| 静态段 `versions` 路由改挂 `:id` 下避免同层 | gin 静态段优先于参数段，同层无冲突（已用 gin v1.12.0 注册 13 条路由冒烟验证）；且与数据映射 `mappings/versions/:vid` 形态一致 |

## 验收标准

- [x] `go build ./...` 绿；`go vet` 触及包无输出
- [x] 聚焦测试绿：`go test ./internal/platformsdk/... ./internal/biz/devmodel/... ./internal/data/devmodel/... ./internal/service/devmodel/...`（新增 3 测：DeleteTMNode 404 透传、UpdateTMDraft schema 往返含 min 指针零值与 ℃ 字符保留、ResolveTM 转换）
- [x] gin v1.12.0 路由形态冒烟：13 条路由注册无 panic（静态段 versions 与 :id 同层）
- [ ] 前端 ThingModels.vue 升级（计划阶段 3，未开始）
- [ ] 权限种子运行时自愈验证（计划阶段 4，未开始）

## 风险与后果

- 商户须在平台配 `thing-model:write` scope，漏配 → 平台 403 msg 原样透传。
- 本服务不持久化物模型数据，无本地迁移；回滚=回退代码，种子新增权限码为幂等 upsert。
- `TMSchema` 为 map 结构，要素名为业务主键；前端编辑须保持键唯一性（平台 biz 兜底校验）。

## 交叉链接

- 计划：`aiDoc/plans/completed/2026-10-09-thingmodel-manageable.md`（阶段 2）
- 平台侧真源：`embodied-platform/sdk/types.go`、`embodied-platform/sdk/client.go`（物模型写方法段）。注意：截至本记录时，平台服务端 `/open-api/v1/thing-models` 仅注册 2 条只读路由（`embodied-platform/internal/server/openapi.go:104-107`，handler 在 `openapi_tenant_model.go`），SDK 写方法对应的服务端开放面路由尚未注册——完整管理面目前只在平台自身会话分档（`embodied-platform/internal/server/device.go` protected 组）。本链路按 SDK 契约先行，端到端依赖平台服务端开放面写路由落地。
- 同范式先例：`aiDoc/notes/implemented/architecture/2026-10-09-datamapping-proxy-chain.md`
- 商户收敛口径：`aiDoc/notes/implemented/architecture/2026-10-08-model-merchant-scope-resync.md`
- 契约正文：`aiDoc/contracts/boundary.md` 组件间契约节
