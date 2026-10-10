<!-- last-updated: 2026-10-10 -->
# 租户部门本地域：租户内部数据隔离基础能力（部门树 + 用户多部门归属）

## 背景

用户需求：租户管理端增加租户部门管理，处理租户内部数据隔离（同一租户内不同部门的人员/数据相互隔离）。调研结论：两仓（本仓与 embodied-platform）完全没有部门概念；设备与租户用户账号真源都在平台（本地无表），平台开放面 `ListDevices`/`ListTenantUsers` 仅支持 tenant_id 级过滤——设备级按部门隔离必须平台侧先有部门域，属跨仓工程。用户确认**本仓先行**（组织架构与人员归属本期落地，设备隔离二期）+ **多部门归属**（一人多部门，隔离查询取并集）。

## 决策

**部门域完全本地化**，对齐 2026-10-09 租户 RBAC 下沉先例（认证/账号留平台，授权/组织放本地）：

- **本地两表**（tenant_id=平台租户 ID、user_id=平台 tenant_user ID，无外键，与 tenants.platform_id 投影同口径）：
  - `tenant_depts`：树形自引用（parent_id 0=根），(tenant_id, code) 复合唯一，软删+墓碑改写 code 释放槽位；表名已核对不在 migrateLegacy 清理名单。
  - `tenant_user_dept_binds`：(user_id, dept_id) 复合主键（多部门），替换式物理删除。
- **用例守卫**：创建/更新校验租户投影存在（TenantChecker 依赖倒置，*biztenant.Usecase 满足）；父部门同租户 + 防环（新父祖先链不得经过自身，全量拉平内存上溯）；删除有子部门拒绝、成员绑定事务内级联解除；SetUserDepts 全量替换且部门须与用户同租户（用户归属经平台 GetUser 定位，对齐 SetUserRoles 模式）。
- **管理端端点**（protected+RBAC）：`/api/v1/tenant-depts` GET/POST、`/:id` PUT/DELETE；`/api/v1/tenant-users/:id/depts` PUT。部门列表平表全量返回（含直属成员数），树形组装在前端（对齐菜单/物模型先例）。
- **部门筛选用户走本地交集路径**：`GET /tenant-users?dept_id=` 有值时，本地 binds 反查部门（含后代展开，BFS 内存展开）成员 user_ids → 按部门租户翻页拉取平台全量（100/页）→ 内存交集 → 本地分页；返回 VO 形状与常规路径一致（前端无感）。UserListParams.DeptID 为本地维度，网关不透传开放面。
- **UserVO 回填 depts [{id,name}]**：userDeptMap 批量解析（对齐 userRoleMap 先例，失败不阻断列表）。

### 已知取舍与边界

- **设备级隔离留二期**：门户端按部门过滤设备需平台侧先建部门域 + 开放面过滤维度，届时本表可作为平台部门域的迁移源。
- **交集路径性能受限**：单租户用户量超出常规规模（数千以上）时翻页拉取成本上升；平台侧支持部门维度后应下推过滤（见 boundary.md 标注）。
- **部门筛选与租户筛选联动**：dept_id 自带租户归属，与显式 tenant_id 不符时返回空集（防御跨租户拼参）。
- **删号级联清理（2026-10-10 当日修正）**：初版沿用「角色 binds 孤儿无害、零投影不清理」的先例到部门——但成员计数直接依赖 binds，残留会让 member_count 虚高且与成员列表（平台交集）口径分裂。管理端 `DELETE /tenant-users/:id` 与门户 `DELETE /tenant-api/v1/members/:id` 平台删号成功后级联 `ClearUserDepts`（清理失败仅告警：账号已删成定局，残留只是计数虚高，不应误报删除失败）。平台侧直接删号无回调通道，残留为已知边界。角色 binds 维持不清理（残留仅影响回填，无计数语义）。
- **计数双口径为产品语义**：部门行「直属成员」列与「成员（含子部门）」弹窗数字天然不同（多部门用户各计 1、弹窗跨部门去重），UI 文案已各自标注，保持现状。
- 门户端（tenant-api）本期不加部门能力（成员列表/权限码均不动）。

## 关键落点

- `internal/data/model/tenantdept.go`（两 PO）、`internal/data/data.go`（AutoMigrate + 菜单/按钮种子 menu:tenantDept、tenantDept:*、tenantUser:setDepts）
- `internal/biz/tenantdept/entity.go`（聚合/哨兵/DeptRepo 端口）、`usecase.go`（CRUD+防环+删除守卫+UserIDsUnderDept 后代展开+ClearUserDepts 级联）
- `internal/data/tenantdept/repo.go`（墓碑/唯一冲突映射/替换语义/MemberCounts/DeptNamesByIDs/DeleteUserDepts）+ `repo_test.go`
- `internal/service/tenantdept/service.go`（DTO）；`internal/service/tenantuser/service.go`（depts 回填 + listUsersByDept 交集路径 + SetUserDepts + DeleteUser 级联清理）
- `internal/service/tenantmember/service.go`（Remove 级联清理，增 depts 依赖）+ `service_test.go`（TestRemoveClearsDeptBinds）
- `internal/server/handler_tenantdept.go` + `router.go`（路由组）+ `i18n_errors.go`（tenant_dept.* 四哨兵）
- `pkg/i18n/messages_{zh,en}.go`（错误文案 + menu.menu:tenantDept 双语）
- `migrations/{mysql,postgres,sqlite}.sql`（两表三方言 DDL）
- 前端：`views/tenant/TenantDepts.vue`（树形表格+CRUD+成员弹窗）、`TenantUsers.vue`（depts 列+设置部门弹窗+部门筛选）、`api/{types,index}.ts`、`router/dynamic.ts`、`locales/{zh-CN,en-US}/tenantDept.ts` + tenantUser 部门 key

## 验证

- `go build ./...` 全绿；wire 重新生成（tenantusersvc.NewService 增 depts 依赖、tenantmembersvc.NewService 增 depts 依赖、tenantdeptService 接入 HTTPServer）。
- 新增测试 `data/tenantdept/repo_test.go`：墓碑释放/同码重建、(tenant_id,code) 唯一冲突、绑定替换语义、MemberCounts/DeptNamesByIDs、DeleteUserDepts 级联回落+幂等、更新防环（自身/后代）、删除守卫（有子部门拒绝、清空后放行）、跨租户守卫、UserIDsUnderDept 后代展开（三层树成员分布）——全部通过。
- `service/tenantmember/service_test.go`：TestRemoveClearsDeptBinds（移除级联清理 + 守卫拒绝路径不触发清理）通过；相邻包 `tenantrole`/`appuser` 回归通过。
- 前端 `npm run lint` 0 error、`npm run build`（含 vue-tsc）通过。
