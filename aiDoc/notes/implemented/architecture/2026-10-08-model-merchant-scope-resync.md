<!-- last-updated: 2026-10-08 -->
# 设备相关模块对齐平台型号商户归属收敛（merchant_id 透出 + DeleteModel 404 语义变更）

- 日期：2026-10-08
- 状态：implemented

## 问题

平台侧 2026-10-08 落地「型号商户归属收敛」（平台提交 `bdd1c9b9`，平台决策 `embodied-platform/aiDoc/notes/proposed/architecture/2026-10-08-model-merchant-scope.md`）：型号/物模型节点新增 `merchant_id`（0=通用，>0=商户独立），开放面读=通用+本商户、写仅本商户独立型号（通用/他商户 404），设备查询叠加型号商户可见性，注册 `model_id` 须商户可见。本服务作为接入方需同步契约：SDK/biz/前端类型缺 `merchant_id`，且既有 `DeleteModel` 的 404 幂等放行在新契约下语义错误（404 兼具"不可写"，静默放行会让未生效的删除假装成功）。

## 提案 / 决策

1. **全链路透出 `merchant_id`（纯增量）**：`internal/platformsdk/types.go`（`DeviceModel`/`TMNode`）→ `internal/biz/devmodel`（镜像类型）→ `internal/data/devmodel/repo.go`（转换映射）→ `web/src/api/types.ts` → `DeviceModels.vue` 新增「归属」列。
2. **通用型号本端只读**：`DeviceModels.vue` 对 `merchant_id === 0` 的行不渲染编辑/删除按钮（平台写面已 404，UI 前置收敛避免无效操作）。
3. **`DeleteModel` 移除 404 幂等放行**：404 原样透传（`platformErr` 映射平台 msg），`repo_test.go` 测试反转为断言透传。
4. **设备域链路零改动**：读收敛、注册 `model_id` 可见性校验均在平台服务端完成，本服务纯透传；顺带修复 biz/device、biz/devmodel、service/devmodel 三处包注释漂移（tenant_id 过滤、开放面 HMAC 口径）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 保留 DeleteModel 404 幂等放行，仅靠前端按钮收敛 | API 直调删除通用型号会静默"成功"但型号仍在列表，行为说谎；幂等收益（重复删除不报错）远小于误导代价 |
| 设备域也加本地型号可见性预判 | 平台服务端已统一收敛（GetDeviceScoped），本服务重复判断只会产生口径分叉 |

## 验收标准

- [x] `go build ./...` 通过；`go test ./internal/platformsdk/... ./internal/data/devmodel/... ./internal/biz/device/...` 全绿（含反转后的 DeleteModel 404 透传测试）
- [x] `cd web && npm run build`（含 vue-tsc）通过
- [x] 型号管理页通用型号显示「通用」tag 且无编辑/删除按钮；本商户型号操作不变

## 风险与后果

- 行为变化：重复删除同一型号第二次会收到 404 错误提示（原为静默成功）——属预期收敛；删除通用/他商户型号会得到平台 404 msg 透传
- 回滚：全部增量+单点语义变更，`git revert` 即可；平台侧存量资源全为通用（merchant_id=0），可见性与改造前一致

## 交叉链接

- 平台侧决策：`embodied-platform/aiDoc/notes/proposed/architecture/2026-10-08-model-merchant-scope.md`
- 本仓库契约：`aiDoc/contracts/boundary.md`（组件间契约节）
- 业务记录：`aiDoc/memory/business/2026-10-08-型号商户归属收敛接入.md`
