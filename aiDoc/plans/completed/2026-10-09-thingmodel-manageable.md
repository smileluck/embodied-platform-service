<!-- last-updated: 2026-10-09 -->
# 商户自定义物模型（物模型页升级为可管理）

## 目标

物模型页从只读浏览升级为商户可管理：节点 CRUD + 版本 draft/publish/rollback/delete。通用节点只读；读=通用+本商户（看不到他商户节点，既有收敛）。

## 非目标

- 不动数据映射/型号/设备既有页面
- 不做 base 层治理策略变更（平台 biz 校验为准）
- refresh-inheritance 平台治理操作不接入

## 假设

- 平台开放面写端点先行落地（平台仓库计划 aiDoc/plans/active/2026-10-09-thingmodel-openapi-write.md）

## 影响面

- platformsdk、biz/data/service/server devmodel 上下文、权限种子、前端 ThingModels.vue + locale

## 验收标准

- [ ] `go build ./...` + 聚焦测试绿；`cd web && npm run build` 绿
- [ ] 通用节点在页面上无写操作入口；本商户节点可增删改、发版本
- [ ] check_sync 0 失败；决策记录与 business 记忆入库

## 执行步骤

1. 平台开放面写端点（平台仓库，先落地）
2. 本服务后端代理链路 + thingModel:* 权限码扩展
3. ThingModels.vue 升级（写操作按 merchant_id 门控）
4. 验证与留痕收尾

## 交付摘要

- **阶段 2（后端代理链路，2026-10-09 完成）**：13 端点纯透传（platformsdk 11 写方法镜像 + biz/data/service/server 四层），权限种子新增 thingModel:* 9 码（view/nodeCreate/nodeUpdate/nodeDelete/draft/draftUpdate/publish/rollback/versionDelete，挂 menu:thingModel）；`go build` + 聚焦测试 + gin 路由冒烟全绿。决策记录 `aiDoc/notes/implemented/architecture/2026-10-09-thingmodel-write-proxy.md`，契约条目已入 `aiDoc/contracts/boundary.md`。
- 阶段 1（平台开放面写端点）：平台 SDK 写方法已上线（`sdk/types.go`/`sdk/client.go`），但**截至 2026-10-09 平台服务端 `/open-api/v1/thing-models` 仅注册 2 条只读路由**（`internal/server/openapi.go:104-107`），写路由尚未开放——本服务写面调用会拿到平台 404，端到端待平台服务端补齐。
- 阶段 3（ThingModels.vue 升级，2026-10-09 完成）：节点树行内操作（任意节点可挂本商户子节点；编辑/删除仅本商户节点）+ 顶部新增节点（layer/parent 联动表单）；右侧详情/Schema 解析/版本管理三页签（resolve 合并结果 + chain + inheritance-status 落后告警）；版本管理= published 列表 + localStorage（`tm_draft:<nodeId>`）会话跟踪草稿（开放面无草稿枚举端点；`baseline==='draft'` 且无本地记录时提示孤儿草稿）；新建草稿确认时才创建、可选预填（resolve 基线 − 父层合并结果的减法）；发布/删除/追加式回滚齐备；Schema 编辑器为页面私有组件 `TMSchemaEditor.vue`/`TMSchemaViewer.vue`（三页签结构化表格，不引入 Monaco）；`npm run build` + eslint 0 error 绿。决策记录 `aiDoc/notes/implemented/feature/2026-10-09-thingmodel-draft-local-tracking.md`
- 阶段 4（种子自愈验证）：未开始。
