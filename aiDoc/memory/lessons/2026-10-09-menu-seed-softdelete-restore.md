<!-- last-updated: 2026-10-09 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 菜单种子不复活软删行：种子菜单被误删后永久消失

## 情境

排查「设备中心看不到设备列表页面」：`menu:device` 在 DB 中是软删状态（deleted_at 非空），`ensureSystemMenus` 用 Unscoped 查到即认为存在、不恢复，菜单树查询排除软删行 → 菜单永久消失，重启无效。

## 坑 / 模式

种子幂等逻辑只覆盖「缺失插入」，漏了「软删恢复」分支；而按钮权限种子（`ensureSystemButtonPerms`）有软删恢复。两套种子口径不一致更隐蔽：菜单软删后按钮自愈的父级解析（非 Unscoped 查询）拿不到菜单 ID，按钮 parent_id 落 0 孤儿化，角色管理界面的权限树也挂不上——一个软删引发两处症状。凡是「Unscoped 查询判存在 + 缺失插入」的种子/对账模式，都必须补软删恢复分支。

## 出现次数

1（2026-10-09，menu:device 软删导致设备列表菜单消失 + device:* 按钮孤儿化）

## 状态

pending

## 晋升去向

（待晋升：种子/对账类逻辑评审清单——Unscoped 判存在必须配软删恢复）

## 晋升后复发

0

## 记录日期

2026-10-09
