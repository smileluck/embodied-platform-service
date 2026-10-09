<!-- last-updated: 2026-10-09 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 新表避开 migrateLegacy 仍在清理的旧表名

> 路径：`aiDoc/memory/lessons/2026-10-09-new-table-legacy-name-collision.md`

新增本地表前先查 `migrateLegacy`（`internal/data/data.go`）：那里可能仍有对**已退役旧表**的 `HasTable → DropTable` 幂等清理。若新表沿用旧名，每次启动都会被先建后删，数据反复清空且无报错。

**实例**：2026-10-09 租户 RBAC 本地化的用户-角色绑定表——旧本地表 `tenant_user_roles`（app_user 双角色时代，2026-10-08 退役）仍在 migrateLegacy 清理名单中；新表改名 `tenant_user_role_binds` 规避。清理旧表的代码不能简单删除（未跑过该版本的存量库仍需要它），改名新表是唯一安全解。
