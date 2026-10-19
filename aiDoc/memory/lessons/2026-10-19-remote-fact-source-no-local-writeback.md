<!-- last-updated: 2026-10-19 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 远端事实源字段不可本地改写充刷新

## 情境

用户管理页 `setAdmission`（`web/src/views/system/Users.vue`）开关准入后界面状态列不更新。

## 坑 / 模式

操作成功后只改写了本地投影字段（`row.projection.enabled`），而列表列渲染的是平台实时字段
（`row.admitted`，事实源在远端）——本地改动不代表显示值，界面看似「操作没生效」。
凡列显示值标注「平台/远端事实源」的，操作成功后不能靠改本地字段冒充刷新，
须整表 `load()` 或定向重拉；改本地字段只在「显示值=本地字段」时成立。

## 出现次数

1

## 状态

pending

## 晋升去向

（待第 2 次出现或用户确认后晋升至 frontend-rules.md）

## 晋升后复发

0

## 记录日期

2026-10-19
