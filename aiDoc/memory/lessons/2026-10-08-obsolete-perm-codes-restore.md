<!-- last-updated: 2026-10-08 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 恢复/新增种子菜单与权限点前，先查 obsoletePermCodes 淘汰清单

## 情境

恢复登录日志模块（2026-10-08）：把旧菜单 `menu:loginLog` 与按钮 `log:login:*` 加回 `internal/data/data.go`
的 `systemMenus`/`systemButtonPerms` 种子后，启动时它们仍被 `migrateLegacy` 的 `obsoletePermCodes`
清单当作「已废弃 code」每次启动删除——种子刚插上就被清掉，页面入口永远出不来。

## 坑 / 模式

`obsoletePermCodes` 是「曾经存在、现已移除」code 的淘汰清单，幂等清理存量库。**任何恢复旧功能或
复用旧 code 的变更，必须先把它从淘汰清单移除**，否则 ensure 种子与 migrateLegacy 清理互相打架。
对称地：下线功能时把 code 加进淘汰清单，恢复功能时把它拿出来——同一 code 不能同时出现在两处。

## 出现次数

1（首次建档）

## 状态

pending

## 晋升去向

暂未晋升（次数 1）；若再犯，规则应写入 `aiDoc/modules/module-development.md` 的菜单/权限种子章节。
