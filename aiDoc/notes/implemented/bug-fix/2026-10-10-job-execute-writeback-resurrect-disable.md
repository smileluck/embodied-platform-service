<!-- last-updated: 2026-10-10 -->
# 定时任务执行收尾整行回写：执行期间禁用被复活（重启后恢复自动执行）

## 问题

期望行为「任务禁用后不自动执行、但允许手动执行一次」。排查结论：调度主路径已满足——`Start` 只装载 `ListEnabled`、`reschedule` 移除 entry 后不再回加、`scheduledJob.Run` 每次触发重读库校验 status、`RunOnce` 不检查禁用状态（手动执行放行）。

真正的缺陷在执行收尾：`execute()` 结束时把**触发时读到的任务快照**整行 `repo.Update` 回库（含 name/cron/handler/params/remark/**status**）。若任务执行期间管理员点「禁用」（`SetStatus` 已写 status=0 并移除 cron entry），执行完成后回写把 DB 的 status 覆盖回 1——页面刷新显示「启用」，**服务重启后 `Start()→ListEnabled` 重新装载，任务恢复自动执行**，违反「禁用后不自动执行」。

## 提案 / 决策

执行收尾只回写自己拥有的字段：新增 `Repo.TouchLastRun(ctx, id, at)`（GORM `UpdateColumn("last_run_at", at)`，不触发 updated_at 联动），`execute()` 末尾用它替代整行 `Update`。执行期间发生的启停/编辑不再被快照覆盖。手动执行禁用任务的能力保持不变（`RunOnce` 本就不查状态，前端「执行」按钮对所有行可用）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 收尾前 Find 最新行再整行 Update | 仍有竞态窗口（Find 与 Update 之间发生的禁用依旧被覆盖），且多一次查询；last_run_at 本就只有 execute 一个写者，定向列更新即无竞态 |
| 在 `RunOnce` 加 `ErrDisabled` 拒绝禁用任务手动执行 | 与需求相反（要求允许手动执行）；`ErrDisabled` 是历史半成品残留（全仓库无返回点），保持不接线 |
| 回写时带条件 `WHERE status=快照值` | 把并发控制推给调用方拼条件，不如收窄写字段直接 |

## 验收标准

- [x] 回归测试 `TestExecute_DoesNotResurrectDisableDuringRun`：执行中禁用 → 收尾后 DB status 仍为禁用、last_run_at 已回写（旧实现整行回写会复活，测试可拦截）
- [x] `go build ./...`、`go test ./internal/biz/job/...` 通过

## 风险与后果

- `execute()` 不再回写 name/cron 等——本来就不该由执行收尾写这些字段，无行为损失。
- 单机版语义不变（无分布式锁，多实例各跑一次，见 entity.go 头注）。
- 兄弟仓库 `embodied-platform` 有同构 job 模块（同款收尾整行回写），本仓修复不自动覆盖，需平台侧另行评估。

## 交叉链接

- 代码：`internal/biz/job/usecase.go` `execute`、`internal/biz/job/entity.go` `Repo.TouchLastRun`、`internal/data/job/repo.go`、`internal/biz/job/usecase_test.go`
- 经验：`aiDoc/memory/lessons/2026-10-10-async-task-full-row-writeback.md`
