<!-- last-updated: 2026-10-11 -->
# 设备中心型号列按名称显示

## 问题

用户反馈设备中心（设备管理列表/详情）型号显示不正确：页面只渲染 `#<model_id>` 裸 ID。设备 VO（平台开放面契约）只有 `model_id`，不含型号名称，前端需自行解析。

排查中同时确认「设备状态未 i18n」非本仓库代码问题：状态枚举映射已于 2026-10-10 批次落地，用户看到的裸英文来自另一个目录（`wujie-embodied-ego-platform/web`）占用 28170 的旧 dev server——「改了不生效先查端口监听者」教训的第 2 次出现。

## 提案 / 决策

- 列表页 `views/device/Devices.vue`：型号列表请求去掉 `status: 1` 过滤改拉全量，构建 `modelNameMap`（id → `名称（code）`），型号列按名称渲染、未命中兜底 `#id`；下拉选项仍只取启用态（注册/筛选语义不变）。
- 详情页 `views/device/DeviceDetail.vue`：按 `model_id` 调 `getDeviceModel` 解析名称，失败兜底 `#id`。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 平台 DeviceVO 增加 model_name 字段 | 跨仓契约变更（先平台后 service），成本远超一次前端映射；型号名称仅展示用途 |
| 型号下拉与映射各自发一次请求（启用/全量分开） | 同端点两次请求浪费；一次全量请求前端过滤即可 |
| 下拉也改为全量型号 | 注册设备选禁用型号会被平台拒绝，下拉保持启用态语义不变 |

## 验收标准

- [x] `npx vue-tsc -b` 零错误
- [x] 本仓库 web/ 起独立端口 dev server 浏览器冒烟：列表型号列显示「EgoCap 工控机 M1（EGOCAP-IPC-M1）」、状态列「已激活」、最近活跃 `YYYY-MM-DD HH:mm:ss`；详情页型号字段显示名称（code）

## 风险与后果

- 型号数量超过 page_size=100 时映射不全，超出部分回落 `#id`（现状型号规模 << 100，暂不引入分页拉取）。
- 租户门户（`tenant-portal/Devices.vue` 等）型号列仍显 `#id`：租户端无型号列表端点（/tenant-api/v1 未暴露），需契约新增后再改，本次不动。

## 交叉链接

- 前序：`aiDoc/notes/implemented/bug-fix/2026-10-10-device-pages-i18n-datetime.md`（状态 i18n 与时间格式）
- 教训：`aiDoc/memory/lessons/2026-10-09-debug-server-port-occupancy.md`（第 2 次出现，已晋升 `aiDoc/relations/development-workflow.md`）
- 代码：`web/src/views/device/Devices.vue`、`web/src/views/device/DeviceDetail.vue`
