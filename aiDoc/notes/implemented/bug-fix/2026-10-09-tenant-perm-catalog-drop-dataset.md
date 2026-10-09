<!-- last-updated: 2026-10-09 -->
# 租户角色配权目录撤下 dataset 域（对齐平台移除口径）

> 路径：`aiDoc/notes/implemented/bug-fix/2026-10-09-tenant-perm-catalog-drop-dataset.md`

## 问题

新建租户角色时配权目录仍显示「数据集」分组（`dataset:list/create/upload/delete` 4 码）。来源：租户 RBAC 本地化时本仓注册表（`internal/biz/tenantuser/permcatalog.go`）为衔接平台同日变更，把平台已撤下的 dataset 域「带回」凑齐原 27 码目录。

## 决策

撤下 dataset 域 4 码，与平台口径对齐（平台 `a5c0f644`：数据集上传收敛为设备端专用 + 管理面，租户门户 dataset 权限域四码与存量 `tenant_role_perms` dataset 行一并清除）。租户门户没有任何数据集功能——这 4 个码不挂任何路由，配了也无所守门，留在目录里只是误导。

初版「带回」的判断（dataset 是商户端业务域、目录应含它）不成立：业务域归属论证的是「码定义在哪个仓」，前提是功能存在；功能已收敛后码即失去对象。

## 改动

- `internal/biz/tenantuser/permcatalog.go`：删 `PermDataset*` 4 常量与 Catalog 4 行（23 码），注释口径更新
- `web/src/locales/{zh-CN,en-US}/tenantRole.ts`：删 dataset 分组与 4 词条；头注释「权限码来自平台目录」更正为本地注册表
- `web/src/api/types.ts`：`TenantUserPermDef` group 注释去掉 dataset

## 影响面

- 存量数据：本地 `tenant_role_perms` 无 dataset 行（已核实为空），无需数据迁移；`NormalizePerms` 对存量角色编辑无影响
- 平台侧：注册表推送为只增不删，若平台 `tenant_perm_defs` 已存本商户 dataset 码则残留，但授权已本地化（`RequireTenantPerm` 读本地三表），平台侧残码无实际效力；治理入口属平台管理面后续事项（见动态化计划遗留事项）
- 同步更新：`../architecture/2026-10-09-tenant-perm-catalog-dynamic.md`（27→23 码表述）、`aiDoc/plans/completed/2026-10-09-tenant-perm-catalog-dynamic.md`（偏差条目）

## 验证

- `go build ./...` + `go test ./internal/server/middleware/...`：passed
- `cd web && npx vue-tsc --noEmit`：passed
- `grep -rn dataset web/src/`：无残留
- 用户页面实测（新建租户角色对话框不再出现数据集分组）：not-run（待用户刷新确认）
