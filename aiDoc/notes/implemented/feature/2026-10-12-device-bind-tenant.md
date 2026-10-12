<!-- last-updated: 2026-10-12 -->
# 设备改绑租户：开放面新端点 + 本仓代理链

> 路径：`aiDoc/notes/implemented/feature/2026-10-12-device-bind-tenant.md`

## 问题

设备注册时绑定租户后无法调整归属：录入错误或租户结构调整时只能删除重注册（丢失设备身份与历史数据）。需要平台开放面提供改绑端点，本仓管理端代理暴露。

## 提案 / 决策

跨仓新增契约端点，本仓按既有注册链模式纯代理：

- **平台侧**（embodied-platform 仓并行实现，本记录只锚定契约）：`PUT /open-api/v1/devices/:id/tenant`，scope `device:bind-tenant`；body `{tenant_id}` 须在商户绑定租户集内否则 403（`ErrTenantNotInScope`）；设备不在本商户数据范围（含他商户独立型号下设备）404；改绑后设备 `merchant_id` 快照同步刷新
- **本仓代理链**：`platformsdk.Client.BindDeviceTenant` → biz `device.Gateway.BindDeviceTenant`（`GatewayAdapter` 纯转发）→ `Usecase.BindTenant` → service `BindTenantRequest` → handler `bindDeviceTenant` → `PUT /api/v1/devices/:id/tenant`
- **校验与补链策略复用注册链**：先 `TenantReader.TenantAvailable` 本地校验（未同步/停用 → `ErrTenantNotSynced` 400）；平台 403（本地租户镜像悬挂）时 `TenantRelinker.RelinkByPlatformID` 补链重建后重试一次（LinkOrCreate 幂等，scope 拒绝等其他 403 重试无害）
- **RBAC**：新权限点 `device:bind`（种子 `PUT /api/v1/devices/*/tenant`，随种子流程绑商户管理员角色），与 `device:register` 同口径单列，不复用粗粒度点
- **前端**：设备列表操作列「绑定租户」（`device:bind` 控显隐），弹窗租户下拉复用已同步启用租户集并回填当前租户

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 复用 `device:register` 权限点控改绑 | 注册与改绑是不同风险动作（改绑影响存量设备归属），粒度应单列；种子一码一 path 的既定口径也不允许跨 path 复用 |
| 本仓侧跳过本地租户校验直透平台 | 与注册链不一致会给出更差的错误体验（平台兜底 msg 不如 `ErrTenantNotSynced` 明确），且丢失悬挂镜像自动补链自愈能力 |

## 验收标准

- [x] `go build ./...` 通过；改文件 `gofmt -l` 为空
- [x] `cd web && npm run build`（含 vue-tsc）通过
- [x] 设备列表操作列按 `device:bind` 权限显隐；改绑成功刷新列表，失败透传平台 msg
- [x] 平台 403/404 语义与本仓错误映射一致（404 → `device.not_found`）

## 风险与后果

- **迁移**：存量商户需在平台管理端为 appKey 补勾 `device:bind-tenant` scope，否则开放面 403（与本仓 RBAC `device:bind` 是两个独立闸门，两侧都要放）；`device:bind` 种子权限点随启动幂等补齐并绑商户管理员角色，其他角色需在角色管理手动勾选
- **回滚**：本仓回滚=撤路由+前端入口即可（无本地表、无数据迁移）；平台侧端点保留不伤人（无调用方即闲置）
- 改绑不校验设备状态（禁用/退役设备也可改绑），治理策略如需收紧由平台侧加规则，本仓代理链天然跟随

## 交叉链接

- 契约：`aiDoc/contracts/boundary.md`（开放面节 + web-api 节）
- 业务记录：`aiDoc/memory/business/2026-10-12-设备列表绑定租户.md`
- 注册链先例（补链重试模式来源）：`internal/biz/device/device.go:Register`
