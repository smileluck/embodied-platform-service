<!-- last-updated: 2026-10-10 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 异步任务收尾整行 Update 回写快照——覆盖执行期间并发的启停/编辑

## 情境

定时任务 `execute()`（`internal/biz/job/usecase.go`）结束时把触发时读到的任务整行 `repo.Update` 回库（含 status）；执行期间管理员禁用了该任务，收尾回写把 status 覆盖回启用，服务重启后任务恢复自动执行。

## 坑 / 模式

长耗时异步流程收尾回写时，持有的是**开始时的快照**，整行/大对象 Update 会把执行期间其他写者落库的变更悄悄回滚（lost update）。规则：收尾只回写该流程自己拥有的字段（定向 `UpdateColumn`/列集合），绝不携带快照里与本流程无关的字段；快照读-改-写整行仅适用于持有乐观锁或独占写者的场景。

## 出现次数

1（2026-10-10 定时任务禁用复活）。

## 状态

pending。

## 晋升去向

若复发，候选正文：`aiDoc/modules/architecture-rules.md` 数据访问小节。

## 晋升后复发

无复发保持 0。

## 记录日期

2026-10-10
