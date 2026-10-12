<!-- last-updated: 2026-10-12 -->
# 全站时间格式统一：前端 formatDateTime 收口 + 后端 timex 单源

## 问题

系统中时间展示格式不统一：后端 log/blacklist/tenant/export 等服务输出 `2006-01-02 15:04:05` 字符串，其余大量接口透出 RFC3339；前端约 12 个视图各自用 `slice(0,16/19).replace('T',' ')` 手工截断（精度有到分钟有到秒，两列并排时肉眼可见不一致），个别列裸显 RFC3339。`aiDoc/frontend/frontend-utils.md` 已有「时间展示必须 formatDateTime」规则，但代码大面积漂移——规则无机械检测手段，批量修复后（2026-10-10 批次修过设备页）仍不断有漏网。

## 提案 / 决策

1. **展示归一在前端收口**：一切用户可见时间戳统一 `web/src/utils/datetime.ts:formatDateTime`（输出 `yyyy-MM-dd HH:mm:ss`）。该函数对 RFC3339 与已格式化字符串均安全（`new Date` 双格式可解析），因此无需后端配合即可全站归一。
2. **后端格式串单源化**：新增 `pkg/timex`（`Layout` 常量 + `Format`，零值输出空串），替换五处重复 `formatTime`（service log/blacklist/tenant + biz tenantdept/tenantuser）、导出链路与 notify 告警消息模板里裸写的 `"2006-01-02 15:04:05"` 字面量；对外输出不变。
3. **明确例外清单**（不追求形式上的全改）：机器消费字段保持原样——token `expires_at` RFC3339、监控 unix 秒、图表轴刻度 HH:mm、平台透传的格式化文本（如 `bound_at`）；数字千分位 `toLocaleString()` 与时间无关。
4. 契约约束回写 `aiDoc/contracts/boundary.md`（时间字段输出与展示节）、`aiDoc/frontend/frontend-utils.md`（强制场景 + 机械自查模式）；lesson 晋升见交叉链接。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 后端全局改写 JSON 时间输出为 `yyyy-MM-dd HH:mm:ss`（自定义 Time 类型 MarshalJSON） | 丢失时区信息；涉及全部 DTO/实体的大范围契约变更，而前端 `formatDateTime` 已能归一，收益不成比例 |
| 逐页保留原有精度（分钟列不放开到秒） | 与「统一格式」诉求直接矛盾，且同屏两列精度不一致正是本次要消除的症状 |
| 顺带把 `internal/service/devmodel/mapping.go` 的既有 gofmt 漂移修掉 | 与本次变更无关，混入会扩大评审面；单独处理 |

## 验收标准

- [x] `rg "slice\(0, 1[69]\)\.replace\('T'"` 在 `web/src`（generated 除外）零命中
- [x] `rg "toLocaleTimeString"` 仅剩图表轴刻度一处（ServerMonitor X 轴，属例外清单）
- [x] `rg '"2006-01-02 15:04:05"' internal pkg` 仅剩 `pkg/timex/timex.go` 定义一处
- [x] `go build ./...` 通过；`go test ./pkg/timex/` 通过（锁定零值空串 + 格式）
- [x] `vue-tsc -b` 与 `vite build` 通过
- [x] 后端原有已格式化接口输出不变（log/tenant/blacklist/export 行为等价替换）

## 风险与后果

- RFC3339 经 `new Date` 按浏览器时区转换：浏览器与服务器时区不一致时，RFC3339 来源与后端已格式化来源显示的时区口径不同；当前部署同为 +08:00，无实际偏差，属已知边界
- 部分时间列从分钟精度放开到秒精度，列宽统一调 170；窄屏下表格横向滚动可能略增
- 新增 `pkg/timex` 为后续所有直显时间字段的唯一格式来源，业务代码禁止再写格式字面量（约束已回写 boundary.md）

## 交叉链接

- 业务记忆：`aiDoc/memory/business/2026-10-12-全站时间格式统一.md`
- 约束正文：`aiDoc/contracts/boundary.md`「时间字段输出与展示」、`aiDoc/frontend/frontend-utils.md`「强制使用场景清单」
- 晋升 lesson：`aiDoc/memory/lessons/2026-10-12-format-drift-no-mechanical-gate.md`
- 相关先行变更：`aiDoc/memory/business/2026-10-10-四项界面缺陷修复批次.md`（第一轮时间格式统一，仅覆盖部分页面）
