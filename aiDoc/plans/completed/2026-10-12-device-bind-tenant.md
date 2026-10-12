<!-- last-updated: 2026-10-12 -->
# 变更计划：设备列表绑定租户（改绑）

> 路径：`aiDoc/plans/completed/2026-10-12-device-bind-tenant.md`（已完成归档）

## 目标

管理端设备列表支持单台设备改绑租户：平台开放面 `PUT /open-api/v1/devices/:id/tenant`（embodied-platform 仓并行实现）+ 本仓代理链（SDK→biz→service→handler→路由+RBAC `device:bind`）+ 前端操作列改绑弹窗，契约/记忆/决策文档同步留痕。

## 非目标

- 不改平台仓 `../embodied-platform`（由另一代理并行实现，本仓只按既定契约写客户端调用）
- 不做批量改绑、不做设备删除/退役等其他设备写面
- 不新增本地设备表（纯代理立场不变）

## 假设

- 平台端点契约：`PUT /open-api/v1/devices/:id/tenant`，scope `device:bind-tenant`，body `{tenant_id}`；403=租户不在商户绑定集，404=设备不在本商户数据范围；返回改绑后设备
- 改绑后设备 `merchant_id` 快照由平台同步刷新，本仓无本地设备数据需迁移

## 影响面

- 后端代理链：`internal/platformsdk/`、`internal/biz/device/`、`internal/data/device/`、`internal/service/device/`、`internal/server/`（device.go/router.go）、`internal/data/data.go`（权限种子）
- 前端：`web/src/api/index.ts`、`web/src/views/device/Devices.vue`、`web/src/locales/{zh-CN,en-US}/device.ts`
- 契约文档：`aiDoc/contracts/boundary.md`（两侧：开放面端点 + 管理端端点）

## 验收标准

- [x] `go build ./...` 通过，所改 Go 文件 `gofmt -l` 为空
- [x] `go test ./internal/biz/device/... ./internal/service/device/... ./internal/server/...` 聚焦测试（无测试文件照实报告）
- [x] `cd web && npm run build`（含 vue-tsc）通过
- [x] boundary.md / business 记忆 / 决策记录 / 索引同步更新

## 工作项

| # | 工作项 | owner | 依赖 | 建议写范围 | 验证命令 | 状态 |
|---|---|---|---|---|---|---|
| 1 | 平台端点实现 | 平台仓代理 | 无 | `../embodied-platform`（本计划不动） | 平台仓自验 | 并行进行中 |
| 2 | SDK 客户端 + 类型 | 本仓代理 | 契约既定 | `internal/platformsdk/` | `go build ./internal/platformsdk/` | 已完成 |
| 3 | biz/data/service/server 代理链 + RBAC 种子 | 本仓代理 | #2 | `internal/{biz,data,service,server}/` | `go build ./...` | 已完成 |
| 4 | 前端 api + 设备页改绑弹窗 + i18n | 本仓代理 | #3 | `web/src/` | `npm run build` | 已完成 |
| 5 | 契约/记忆/决策文档留痕 | 本仓代理 | #3 #4 | `aiDoc/` | 漂移自检 | 已完成 |

## 验证命令

```bash
go build ./...
go test ./internal/biz/device/... ./internal/service/device/... ./internal/server/...
cd web && npm run build
gofmt -l <所改 Go 文件>
```

## 回滚 / 迁移

- 无数据迁移（本仓不落设备表）；`device:bind` 权限种子随启动幂等补齐，存量商户需在平台管理端为 appKey 补勾 `device:bind-tenant` scope
- 回滚=撤本仓路由 + 前端入口（`git revert` 单提交即可）；平台端点保留无害

## 当前状态

全部工作项完成，验证见交付摘要。

## 交付摘要（完成时填写）

- 交付：后端代理链 8 文件（SDK scope 常量 `device:bind-tenant` + `BindDeviceTenantRequest`/`BindDeviceTenant`、Gateway/Adapter/Usecase.BindTenant/Service/Handler/路由 `PUT /api/v1/devices/:id/tenant`、种子 `device:bind`）；前端 4 文件（`bindDeviceTenant` api、Devices.vue 操作列+改绑弹窗、双语 locale）
- 验证：`go build ./...` passed；聚焦测试三包均 no test files（skipped，仓库该三处本无测试）；`npm run build`（vue-tsc + vite）passed；所改 Go 文件 `gofmt -l` 为空（types.go 顺带清掉存量 EOF 空行）
- 偏差：types.go 顺带修复了存量 gofmt 尾行问题；wire 无需重生成（无新构造依赖），i18n 复用现有哨兵与 `device.not_found` 未新增 key，均符合预期
- 遗留：平台端点由并行代理实现，端到端联调待双侧合并后冒烟；存量商户 scope 补勾为运维动作
