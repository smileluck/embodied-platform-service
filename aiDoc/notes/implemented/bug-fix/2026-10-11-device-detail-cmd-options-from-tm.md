<!-- last-updated: 2026-10-11 -->
# 设备详情命令下发类型从物模型服务生成

## 问题

设备详情「命令下发」的命令类型下拉硬编码仅 `emergency:estop` 一项（`web/src/views/device/DeviceDetail.vue` 的 `cmdTypeOptions` 常量）。设备型号已绑定物模型节点与版本，物模型 schema 中定义的服务（`services`，即设备可下发的命令）完全没有被读取——用户无法从界面选到物模型定义的真实命令，只能靠 tag 手输。

## 提案 / 决策

设备详情加载型号（既有 `getDeviceModel`）后，用返回的 `tm_node_id` + `tm_version_id` 调 `resolveThingModel` 解析合并 schema，命令类型选项按 `schema.services` 的名称键生成；内置急停 `emergency:estop` 作为兜底项补位（物模型未定义时），select 保持 `filterable tag` 允许手输。

关键点：

- **pin 型号绑定版本**：resolve 传 `tm_version_id`（而非缺省基线），保证选项与型号发布时绑定的物模型一致，基线后续演进不会漂移出该型号未定义的命令。
- **急停兜底不删**：`emergency:estop` 是平台内置命令，与物模型无关；物模型未定义它时补位，已定义时去重。
- **失败静默降级**：型号或物模型解析失败时仅剩急停 + 手输，不阻塞详情页（与页面既有「影子/型号名」失败兜底风格一致）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 后端新增「设备可下发命令」聚合接口 | 单页面消费、两跳接口（device-models → thing-models/resolve）前端已可组合，无复用面，不值得加契约 |
| resolve 不传 version_id（取节点基线） | 型号绑定的是具体已发布版本；基线后续发布会改变命令集合，与型号实际契约脱节 |

## 验收标准

- [x] `npm run build`（vue-tsc -b）零错误
- [x] `npm run lint` 0 error（DeviceDetail.vue 仅存量 any warning）
- [ ] 登录后进入设备详情 → 命令下发，下拉出现型号绑定物模型的服务名 + `emergency:estop`（GUI 冒烟阻塞：admin 平台密码有效但本系统准入未开通，无成员账号凭据，待有凭据环境补验）

## 风险与后果

- 设备型号若未绑物模型（`tm_node_id=0`）或绑定版本已删，选项回退为仅急停 + 手输，不报错。
- 型号更新绑定版本后需刷新页面才刷新选项（详情页每次进入重新拉取，可接受）。
- 物模型服务名即命令 `command_type` 值，与平台侧命令路由约定一致（服务名主键）。

## 交叉链接

- 业务需求：[../../memory/business/2026-10-11-命令下发读取物模型服务.md](../../memory/business/2026-10-11-命令下发读取物模型服务.md)
- 物模型契约：`web/src/api/types.ts`（`TMSchema.services` / `TMResolveResult`）
- 相关：notes/implemented/bug-fix/2026-10-11-device-model-name-display.md（同页型号名解析兜底先例）
