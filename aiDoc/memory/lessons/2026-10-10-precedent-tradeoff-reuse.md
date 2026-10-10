<!-- last-updated: 2026-10-10 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 先例取舍沿用过界：无害结论要按消费方重新评估

> 路径：`aiDoc/memory/lessons/2026-10-10-precedent-tradeoff-reuse.md`

## 情境

2026-10-10 租户部门域实现：沿用 2026-10-09 租户 RBAC「删号后本地 binds 成孤儿无害、零投影原则不清理」的先例到部门绑定，用户反馈成员计数可疑后自查发现 bug。

## 坑 / 模式

把 A 场景的取舍结论直接复制到 B 场景时，只看了「数据形态相同」（同样是孤儿绑定行），没有重查「消费方是否相同」：角色 binds 的孤儿只影响列表回填（显示一个不存在的 role_id，无害）；部门 binds 的孤儿直接虚增 member_count，且与成员列表（平台交集过滤）口径分裂——同一功能的两个数字对不上。**沿用先例前必须列出新场景的全部消费方，逐一确认「残留无害」仍成立；有计数/统计/聚合消费的，残留即有形伤害，需级联清理。**

## 出现次数

1（2026-10-10 部门 member_count 虚高，当日修正：删号两入口级联 ClearUserDepts，见 `notes/implemented/architecture/2026-10-10-tenant-dept-local-domain.md` 已知取舍节）

## 状态

pending

## 晋升去向

（pending；若复发考虑晋升至 `modules/architecture-rules.md` 数据层约束：绑定类残留数据的清理义务按消费方判定）

## 晋升后复发

0

## 记录日期

2026-10-10
