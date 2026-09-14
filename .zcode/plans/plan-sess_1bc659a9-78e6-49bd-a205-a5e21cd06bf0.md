# 权限与菜单底层合并:按钮权限挂菜单方案

## 目标
把权限管理融入菜单管理:每个菜单下可挂"按钮权限点"(`type='button'`),按钮的 `code` 控制前端显隐,可选绑定 `method/path` 同时参与后端 RBAC 校验。前端收敛为一个页面。`api` 类型退役(数据迁移为 button)。

## 一、后端改动

### 1. `internal/biz/permission/entity.go`
- Type 枚举加 `TypeButton Type = "button"`;保留 `TypeAPI` 仅作存量兼容(注释标记 deprecated)
- `Match()`(30-51 行):改为「非 menu 且 method/path 非空即参与匹配」——button(及存量未迁移 api)绑定了接口就生效
- `BuildMenuTree` 不变(只组 menu,button 不进侧边栏/路由)

### 2. `internal/biz/permission/usecase.go`
- `Delete`:删除前查 `parent_id = id` 的子节点,存在则返回业务错误"请先删除子级节点"
- `Update`:放开 `parent_id` 修改,加防环校验(新父级不能是自身或其后代,沿 parent 链上溯判断)

### 3. `internal/service/permission/service.go`
- `CreateRequest.Type` 校验 `oneof=api menu` → `oneof=menu button`
- `UpdateRequest` 增加 `parent_id`(可选)

### 4. `internal/data/data.go`(seed + 存量库幂等迁移)
- seed 调整:
  - ID=1 "全部权限" `Type: api` → `button`(仍 `*`/`*`,超管不受影响)
  - 删除 ID=113 "权限管理"(`menu:permission`)菜单项
  - ID=114 名称"菜单管理" → "菜单与权限"
  - 新增示例按钮权限点:用户管理下 `user:create` / `user:delete`(挂 ID 111 下),供演示
- AutoMigrate 后追加幂等迁移(每次启动执行,兼容 mysql/postgres/sqlite):
  - `UPDATE permissions SET type='button' WHERE type='api'`
  - 删除 `code='menu:permission'` 记录及其 `role_permissions` 关联
  - `UPDATE` `menu:menu` 记录名称为"菜单与权限"
- 顺带给角色 1 补绑示例按钮的 `role_permissions`(仅空库 seed 分支)

### 5. `migrations/mysql.sql` / `postgres.sql` / `sqlite.sql`
- 同步表结构注释中 type 枚举说明(`menu/button`,注明 api 已废弃由迁移自动转换)

## 二、前端改动

### 6. `web/src/views/system/Menus.vue` — 升级为统一的"菜单与权限"页
- 标题改"菜单与权限";数据拉取不再按 type 过滤(拉全部)
- 树构建:menu 按 parent_id 组树;button 挂到所属菜单 children 下;`parent_id=0` 的 button(如存量迁移的)显示在树根
- 表格列:类型(menu/button 用 NTag 区分)、名称、编码、路由/接口(method+path)、图标、排序、操作
- 操作列:menu 行 → 编辑/加子菜单/加权限点/删除;button 行 → 编辑/删除
- 表单双模式:
  - menu 表单:现有字段;**父级改用 n-tree-select**(选项为菜单树,可嵌套选任意层级;编辑时排除自身及后代)——修复"只能建两级"
  - button 表单:名称、编码、可选 Method/接口路径(说明:填写后同时控制后端接口)
- 删除:前端预检有子节点则提示阻止;后端兜底报错
- 图标选择器保留

### 7. 删除 `web/src/views/system/Permissions.vue`
- `web/src/router/dynamic.ts` viewModules 移除 `menu:permission` 映射(存量库该菜单已被迁移删除,不会 404)

### 8. `web/src/views/system/Roles.vue` — 授权树简化
- `openPerms`(105-122 行):去掉"接口权限"虚拟节点(-1);menu+button 统一按 parent_id 组树(button 自然挂在菜单下)
- `savePerms` 去掉 `filter(k => k > 0)`
- 树节点 label 可加类型徽标(菜单/按钮)辅助辨认

### 9. `web/src/views/system/Users.vue` — 按钮权限示范
- 新增/删除按钮接 `v-permission="['user:create']"` / `['user:delete']`(与 seed 示例对齐,验证完整链路)

### 10. `web/src/api/types.ts`
- Permission 类型注释更新:type 为 `menu | button`

## 三、不动的东西(零改动)
- `/auth/profile` 下发全部 code(自动含 button)、`v-permission` 指令、`stores/user.ts`
- RBAC 中间件缓存、`role_permissions` 关联表、`/api/v1/permissions` 路由
- 超管 `all` 通配权限迁移后照常生效;侧边栏路由生成逻辑不变

## 四、验证
1. `go build ./...` + `go vet ./...`
2. `cd web && npm run build`(air 托管 dist,rebuild 后强刷)
3. 删本地 sqlite/重置库验证 seed;或直接启动验证存量迁移(api→button、菜单入口合并)
4. 浏览器登录 admin 验证:菜单与权限页树形展示+增删改、按钮创建/绑定接口、Roles 授权树勾选按钮、Users 页按钮显隐随角色变化