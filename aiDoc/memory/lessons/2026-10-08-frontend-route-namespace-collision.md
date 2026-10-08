<!-- last-updated: 2026-10-08 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 新增前端路由区先查既有菜单路径：前缀分流会误伤同前缀页面

## 情境

租户端路由区最初用 `/tenant` 前缀 + 守卫按 `to.path.startsWith('/tenant')` 分流（2026-10-08，
`web/src/router/index.ts`）。而管理端「租户中心」菜单路径本就是 `/tenant/tenants`、
`/tenant/app-users`（种子见 `internal/data/data.go:ensureSystemMenus`）——管理员点「租户管理」
被守卫误判为租户端访问，因无租户 token 被踢到租户端登录页。

## 坑 / 模式

在既有 SPA 里新增「同应用多端路由区」时：
1. 先 grep 既有菜单/路由的真实路径（后端菜单种子 `ensureSystemMenus` + `dynamic.ts:viewModules`），新路由区前缀必须与之**物理不重叠**；
2. 守卫分流优先用路由自身标记（命名路由 name / `to.name` 前缀、matched 链上的 meta），而不是裸路径 `startsWith`——后者会被任意同前缀页面误触发。

本次修复：路由区整体迁到 `/tenant-portal/*`（登录页与壳同步迁移），前缀与管理端无交集。

## 出现次数

1（首次建档）

## 状态

pending

## 晋升去向

暂未晋升（次数 1）；若再犯，规则应写入 `aiDoc/frontend/frontend-rules.md`（新增页面/路由章节）。

## 晋升后复发

无（0）。

## 记录日期

2026-10-08
