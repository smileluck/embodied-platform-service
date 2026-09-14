# SmileX Admin 五项改进计划

## 1. 登录页美化（web/src/views/login/Login.vue）
重写为更有高级感的设计（纯 CSS，无新依赖）：
- 深色渐变背景（深蓝紫夜色系）+ 两个模糊光斑（blur 圆形 div）营造氛围
- 左右分栏或居中玻璃拟态卡片：`backdrop-filter: blur`、半透明白、细边框、柔和阴影
- 卡片头部加 Logo 标识 + 标题/副标题文案，输入框圆润化，登录按钮加渐变色和 hover 过渡
- 底部版权小字

## 2. 布局层次感（web/src/layout/AdminLayout.vue）
- 侧边栏：深色模式配色（dark theme 局部应用）、渐变底色、logo 区加分隔线
- 内容区：`n-layout-content` 背景改为浅灰 `#f5f7fa`，页面卡片浮于其上
- Header：白底、面包屑 + 用户下拉，加轻微阴影 `box-shadow`
- `router-view` 加淡入过渡动画（`<transition name="fade">`）

## 3. 修复侧边栏无法点击（AdminLayout.vue:9）
根因：`n-menu` 没有绑定任何导航逻辑。
- 给 `n-menu` 添加 `@update:value="(k) => router.push(String(k))"`
- 无子菜单的父级菜单（有 children 的分组）不受影响；若后端菜单存在无 component 的节点则不跳转（仅叶子节点有路由，非叶子 key 不在已注册路由中，push 前可简单判断或依赖 catch-all 兜底）

## 4. 区分静态路由与动态路由（web/src/router/index.ts）
- 将 `viewModules`、`menuToRoutes` 及动态注册逻辑拆到新文件 `web/src/router/dynamic.ts`
- `index.ts` 只保留静态路由表（`/login`、`layout-root`、404 兜底）+ 全局守卫；守卫内调用 `dynamic.ts` 导出的 `setupDynamicRoutes()`
- 纯重构，行为不变

## 5. admin 超管账号保护（仅 admin 自己可操作）
后端（internal/biz/user/usecase.go、internal/server/http.go）：
- 在 `Update` / `SetRoles` / `ResetPassword` / `Delete` 中增加保护：目标 id==1 且操作者（从 ctx 中间件注入的 `middleware.Subject(c)`）不是 id==1 时，返回错误（如 "无权操作超级管理员"）；操作者上下文需从 handler 层通过 `context.WithValue` 传入 biz 层（在 http.go 各 handler 中取 `middleware.Subject(c).UserID` 传入）
- `Delete` 保持仅 admin 自己可删（实际上仍是 id==1 拒绝所有人，admin 自己也走 id==1 判断——按所选规则改为"操作者==目标==1 时允许"，即 admin 可删自己；如需保守可保留禁止删除，实现时采取：删除仍禁止 id==1，其余操作仅 admin 本人可用）※采用保守方案：删除仍一律禁止，编辑/重置密码/分配角色仅 admin 本人可操作
- 定义统一错误如 `ErrSuperAdminProtected`，handler 层返回 403

前端（web/src/views/system/Users.vue）：
- 操作列对 `row.id === 1` 且当前登录用户非 id 1 的行隐藏"编辑/重置密码/删除"按钮（当前用户 id 从 `userStore.user?.id` 获取）
- 删除操作补加 `useDialog` 确认弹窗（顺带修复无确认直接删除的问题）

## 验证
- 前端 `npm run build`（或 vue-tsc）类型检查通过
- 后端 `go build ./...` 通过

不涉及数据库结构变更。