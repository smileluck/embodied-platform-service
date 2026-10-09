# 租户 RBAC 本地化数据迁移（2026-10-09，一次性）

把平台库（embodied-platform）的租户角色三表迁到商户端服务库（embodied-platform-service）本地三表。
角色 id 原值保留（本地表为空表直插）；`tenant_id` 为平台租户 ID、`user_id` 为平台 tenant_user ID，原值直填。

**执行顺序**：先在 service 库跑本脚本 → 部署平台（migrateLegacy 会 DropTable 平台四表，**先迁移后部署平台，顺序不可反**）→ 部署 service（AutoMigrate 建本地三表）。

注意：service 新代码首次启动时 AutoMigrate 自动建表，本脚本在**新 service 代码部署前**执行需要先手工建表（DDL 见文末），或改为部署 service 后执行（平台尚未部署前平台表仍在，数据源有效）——推荐顺序：**部署 service（本地三表已建、读写本地）→ 执行本脚本灌数据 → 部署平台（清平台表）**。两部署之间角色页短暂显示空数据，属预期窗口。

## MySQL 版（开发库口径；其余方言同构改写）

```sql
-- 0) 目标库：embodied-platform-service（本地三表已由 AutoMigrate 建好）
-- 1) 幂等：先清后插
DELETE FROM tenant_user_role_binds;
DELETE FROM tenant_role_perms;
DELETE FROM tenant_roles;

-- 2) 角色（保留平台原 id；软删行不迁）
INSERT INTO tenant_roles (id, tenant_id, name, code, remark, created_at, updated_at)
SELECT id, tenant_id, name, code, remark, created_at, updated_at
FROM <平台库>.tenant_roles WHERE deleted_at IS NULL;

-- 3) 权限点
INSERT INTO tenant_role_perms (role_id, perm_code, created_at)
SELECT role_id, perm_code, created_at FROM <平台库>.tenant_role_perms
WHERE role_id IN (SELECT id FROM tenant_roles WHERE deleted_at IS NULL);

-- 4) 用户绑定（user_id=平台 tenant_user ID）
INSERT INTO tenant_user_role_binds (user_id, role_id, created_at)
SELECT user_id, role_id, created_at FROM <平台库>.tenant_user_roles
WHERE role_id IN (SELECT id FROM <平台_roles>.tenant_roles WHERE deleted_at IS NULL);
-- 上述子查询需在 DELETE 前留快照，或直接改用平台库限定：
--   WHERE role_id IN (SELECT id FROM <平台库>.tenant_roles WHERE deleted_at IS NULL)

-- 5) 行数校对（三组数字应一致）
SELECT
  (SELECT COUNT(*) FROM <平台库>.tenant_roles WHERE deleted_at IS NULL) AS platform_roles,
  (SELECT COUNT(*) FROM tenant_roles) AS local_roles,
  (SELECT COUNT(*) FROM <平台库>.tenant_role_perms) AS platform_perms,
  (SELECT COUNT(*) FROM tenant_role_perms) AS local_perms,
  (SELECT COUNT(*) FROM <平台库>.tenant_user_roles) AS platform_binds,
  (SELECT COUNT(*) FROM tenant_user_role_binds) AS local_binds;
```

## 回退

开发库数据量极小（商户 6 测试数据），可接受直接删除本地三表数据后用新管理台手工重建角色。

## 首启前手工建表（仅当先于 service 部署执行时需要）

DDL 见 `migrations/mysql.sql` 的「租户 RBAC 本地三表」段（sqlite/postgres 同构段在对应文件）。
