<!-- last-updated: 2026-10-09 -->
# 菜单种子补软删恢复分支（menu:device 误删不复活）

## 问题

「设备中心看不到设备列表页面」。根因：`menu:device` 权限行于 2026-10-05 被软删（途径不可考），而 `ensureSystemMenus`（internal/data/data.go）用 Unscoped 按 code 判存在，软删行被视为「已存在」不再插入，菜单树查询又排除软删行——菜单永久消失，重启不自愈。次生症状：按钮自愈（ensureSystemButtonPerms）的父级解析用非 Unscoped 查询拿不到软删菜单，`device:*` 9 个按钮 parent_id 落 0 孤儿化。

## 提案 / 决策

`ensureSystemMenus` 补软删恢复分支：查到软删行时清 deleted_at 并校正种子定义字段（name/type/path/icon/sort/parent_id），与按钮种子的软删恢复同口径。仅软删行走字段校正，存活行不动（用户改名等漂移不覆盖）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 手工 SQL 恢复该行，不改代码 | 治标；同类误删（菜单管理页可删任意菜单）再发生仍不自愈 |
| 菜单管理页禁止删除种子菜单 | 是另一层防护，但与自愈不互斥；单做防护无法修复已坏的存量库 |

## 验收标准

- [x] 重启后 `menu:device` deleted_at 清空、parent_id=8（menu:deviceCenter）；`device:*` 按钮 parent_id 自愈为 9（本地库已验证）
- [x] 角色2（商户管理员）持有 menu:device 绑定（既有绑定未随软删丢失，恢复即生效）

## 风险与后果

- 行为变化：被管理员有意删除的种子菜单会在重启后复活——种子菜单语义本就是「系统保障存在」，与按钮种子口径一致化
- 回滚：git revert；已复活的行如需删除可再走菜单管理（但会再次复活，这是预期语义）

## 交叉链接

- lesson：`aiDoc/memory/lessons/2026-10-09-menu-seed-softdelete-restore.md`
- 关联变更：`aiDoc/plans/completed/2026-10-09-thingmodel-datamapping-pages.md` 阶段 4
