<!-- last-updated: 2026-10-08 -->
# 404/500 页"返回首页"落到第一个可访问菜单

## 问题

404/500 错误页的"返回首页"按钮 `router.push('/')`，而已登录且动态路由已加载（`routesLoaded === true`）时守卫直接放行 `/`，匹配到的 `layout-root` 壳没有自身页面组件，AdminLayout 的 router-view 渲染空白。只有刷新场景（路由未加载，守卫走 `setupDynamicRoutes` 分支）才会把 `/` 重定向到第一个菜单——同一路径在两种状态下行为不一致。

## 提案 / 决策

在 `web/src/router/dynamic.ts` 抽出 `firstAccessiblePath(menus)`：遍历菜单树，dir（仅分组无路由）取其第一个子菜单，返回第一个非 dir 菜单的 path，无菜单兜底 `/`；菜单树本身已按用户权限过滤，树中第一个即"第一个可访问菜单"。守卫（`web/src/router/index.ts`）在已加载状态下对 `to.path === '/'` 同样重定向到该路径，与首载分支口径一致。修复点在守卫而非错误页组件，404/500 两个页面及未来任何跳 `/` 的入口一并覆盖。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 只改 NotFound.vue / ServerError.vue 的 goHome 计算目标 | 修复面窄，遗漏其他跳 `/` 的入口，且 `/` 空壳问题本身仍在 |
| 给 layout-root 挂动态 redirect 函数 | redirect 需访问 Pinia store，注册时机早于 store 可用；守卫里已有 '/' → 首菜单的先例，风格统一 |

## 验收标准

- [x] 已登录、路由已加载时访问未知路径进 404，点"返回首页"落到侧栏第一个菜单页面（非空壳）
- [x] 刷新未知深链接后点"返回首页"同样落到第一个菜单
- [x] `npx vue-tsc -b` 零错误、`npm run lint` 零 error

## 风险与后果

- 无菜单的账号（理论上不存在，RBAC 至少返回一个）会落到 `/` 空壳，行为与改动前一致，无新增风险。
- 租户端不受影响：`/tenant` 分支在守卫中提前 return，`/tenant` 自有 redirect。

## 交叉链接

- 代码：`web/src/router/dynamic.ts`（firstAccessiblePath）、`web/src/router/index.ts`（守卫 '/' 重定向）
- 相关：`aiDoc/frontend/frontend-rules.md` 路由一节（动态路由机制概述）
