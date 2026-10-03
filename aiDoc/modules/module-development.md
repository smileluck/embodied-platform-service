<!-- last-updated: 2026-10-03 -->
# 模块开发指南（module development）

> 新建模块/功能的完整步骤。本项目为 mixed：后端分层服务模块 + 前端功能；步骤与参考文件指向真实路径。

## 设计原则

- 模块自包含：同一限界上下文的文件分别集中在 `internal/{biz,data,service}/<ctx>/` 与 `internal/server/handler_<ctx>.go`
- 遵循现有模式：动手前先读一个既有模块（推荐 `dict`，最完整最小；`tenant` 可看平台同步编排）
- 不引入项目尚未使用的库或模式；确有需要时先写决策记录（见 [../notes/README.md](../notes/README.md)）

## 新建后端模块（限界上下文）

以 dict 为范本逐步：

1. **领域层**：`internal/biz/<ctx>/entity.go` — 实体（json tag snake_case）+ 哨兵错误 `ErrXxx` + `Repo` 接口（方法带 `ctx`）；`usecase.go` — `NewUsecase(repo Repo)` + 用例方法（分页返回 `pagination.Page`）
2. **PO 与仓储**：`internal/data/model/model.go` 增 `XxxPO` + `XxxToPO`/`XxxFromPO` 转换；`internal/data/<ctx>/repo.go` — `NewRepo(d *data.Data)`，错误映射（`mapXxxErr` + `isUniqueViolation`）、`RowsAffected` 检查、`EscapeLike`、软删表配 `ArchiveUniqueColumns`（规则见 [architecture-rules.md](architecture-rules.md) 数据层）
3. **应用服务**：`internal/service/<ctx>/service.go` — 请求 DTO（binding tag）→ biz Input 转换；可缺省字段用指针
4. **handler**：`internal/server/handler_<ctx>.go` — 方法挂 `HTTPServer`；参数错误 `response.BadRequest + i18n.T(ctx, "common.invalid_params")`；业务错误 `response.FailI18n`（新哨兵错误在 `internal/server/i18n_errors.go:errKeys` 注册 key）
5. **注册路由**：`internal/server/router.go:registerRoutes` — protected 组 `v1.Group("/<resources>")` 挂 CRUD；本人数据/登录即可用的接口进 basic 组
6. **依赖注入**：`cmd/server/wire.go` — `bizSet` 增 `biz<ctx>.NewUsecase`、`dataRepoSet` 增 `data<ctx>.NewRepo` 与 `wire.Bind(new(<ctx>.Repo), new(*data<ctx>.repo))`、`serviceSet` 增 `svc.NewService`；跑 `make wire` 重新生成 `wire_gen.go`（生成物不手改）
7. **迁移**：新表/新列同步 `migrations/{mysql,postgres,sqlite}.sql` 三方言；`internal/data/data.go:migrateAndSeed` 登记 AutoMigrate 与种子数据
8. **聚焦测试**：仓储用 sqlite 内存库 + 用例单测（既有范式见 `internal/platformsdk/client_test.go`、`internal/conf/config_env_test.go`）；跑 `go test ./internal/<...>/...`

## 新建前端功能

1. **类型**：`web/src/api/types.ts` 增接口类型（与后端 DTO 字段 snake_case 一致）
2. **API 函数**：`web/src/api/index.ts` 增封装（走 `request` 实例；范本见 `getProfile` 等既有函数）
3. **菜单/权限点**：管理端「菜单管理」建 `menu:<code>` 与按钮权限点（后端路由进 protected 组即受控）
4. **页面**：`web/src/views/<域>/<Page>.vue`（`<script setup>` + TS；表格/表单模式参考 `views/system/Dicts.vue`）
5. **登记路由**：`web/src/router/dynamic.ts:viewModules` 增 `'menu:<code>': () => import('../views/...')`
6. **国际化**：`web/src/locales/zh-CN/` 与 `en-US/` 同步增 key；`npm run lint` 通过

## 跨组件变更顺序（后端 + 前端同改）

契约先行：先改后端 DTO 与 handler → 前端 `types.ts` 与 `api/index.ts` 跟进 → 页面/路由/i18n → `make wire` + `make lint` + `make test` + 前端 `npm run build`。涉及平台开放面的（设备/租户/型号/文件），契约以平台仓库为准并两侧分别留痕（见项目集根 `../../AGENTS.md`）。

## 完成前检查

- [ ] 契约两侧字段与 [../contracts/boundary.md](../contracts/boundary.md) 一致
- [ ] 按影响面跑过聚焦验证（见根 `AGENTS.md` 操作不变量）
- [ ] 若模块带来非平凡决策，已写 `aiDoc/notes/` 决策记录
- [ ] 模块↔页面映射变化已更新 [../relations/system-map.md](../relations/system-map.md)

## 真实参考文件

- 后端最小完整范本：`internal/biz/dict/` + `internal/data/dict/repo.go` + `internal/service/dict/service.go` + `internal/server/handler_dict.go`
- 平台同步编排范本（含开放面调用/回填）：`internal/biz/tenant/` + `internal/data/tenant/`
- 前端页面范本：`web/src/views/system/Dicts.vue`、`web/src/router/dynamic.ts`
