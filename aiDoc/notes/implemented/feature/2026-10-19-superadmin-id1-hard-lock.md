<!-- last-updated: 2026-10-19 -->
# 超管 ID=1 本端硬锁定 + 准入开关后刷新列表

## 问题

2026-10-19 用户提出两点：

1. 用户列表「开启/关闭准入」后列表不刷新——`setAdmission` 只改了 `row.projection.enabled`，
   而准入状态列渲染的是平台实时 `row.admitted`（事实源），本地改动不代表显示值，
   界面上状态标签原地不动，用户无法确认操作生效。
2. 超管（平台用户 ID=1）仍可被操作。既有锁定完全依赖平台下发 `is_super` 标记链路
   （2026-10-16 决策），在「旧平台 + 新 service」或存量投影快照未补齐的组合下标记缺位，
   页面按钮照常渲染、本地哨兵按 false 放行。

## 决策

- **ID=1 硬锁定**：`internal/biz/admission/usecase.go` 新增 `PlatformSuperAdminID = 1`
  （与平台侧 `SuperAdminID` 同口径）。三守卫（`SetEnabledByPlatformUser` /
  `SetRolesByPlatformUser` / `DeleteMember`）命中条件改为「ID=PlatformSuperAdminID 或
  投影 is_super 快照」，且先于任何平台写回；`ListFromPlatform` 行 `IsSuper` 归一化为
  `a.IsSuper || a.ID == PlatformSuperAdminID`，前端按钮隐藏逻辑不变（仍由 is_super 驱动）。
  推翻 2026-10-16 决策中「本地不硬编码平台用户 ID」的结论（见其备选表），
  标记链路保留作为第二识别源。
- **开关后刷新**：`Users.vue` `setAdmission` 成功后改为 `load()` 整表刷新
  （数据源在平台，本地字段不代表显示值），删除原先对 `projection.enabled` 的本地改写。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 维持只走平台 is_super 标记，定位并修复标记链路缺位环节 | 缺位发生在平台侧部署版本/存量数据，非本仓可闭环；用户明确要求 ID=1 不可操作，本端硬兜底即时生效且不依赖部署顺序 |
| 前端自行硬编码 platform_user_id===1 隐藏按钮 | 违反一个事实一个维护家：规则已在后端归一化进 is_super，前端保持标记驱动即可，双处硬编码易漂移 |
| 开关成功后只本地改 row.admitted 不刷新 | admitted 是平台事实源，本地伪造显示值与「刷新快照」语义冲突；整表 load() 成本可忽略且顺带带出并发变更 |

## 验收标准

- [x] `go test ./internal/biz/admission/`（新增：ID=1 无快照三操作拒绝且零平台写回、
  列表行 ID=1 无平台标记仍归一化为超管）
- [x] `go build ./...`、`vue-tsc -b` 通过
- [ ] 联调：超管行（ID=1）无操作按钮；开关准入后列表状态列即时更新

## 风险与后果

- 平台种子语义（超管 ID=1）进入本端常量：平台若变更超管 ID 需两侧同步修改
  （与平台侧 `SuperAdminID` 同口径，变更概率极低）；is_super 标记链路保留，
  多超管场景仍可经标记扩展。
- 无迁移/回滚要求：守卫条件放宽（更严格），列表行仅多标记一行。

## 交叉链接

- 被补充/部分推翻的决策：`2026-10-16-superadmin-member-lock.md`（备选「本地硬编码
  平台用户 ID=1 做守卫」由本记录推翻）
- 代码：`internal/biz/admission/{usecase,usecase_test}.go`、`web/src/views/system/Users.vue`
- 业务需求：`aiDoc/memory/business/2026-10-19-用户列表准入刷新与超管ID1锁定.md`
