<!-- last-updated: 2026-10-05 -->
# 路由重复注册只在启动期暴露：改动 router 后必须做一次启动验证

## 场景

2026-10-05 联调冒烟启动时 panic：`handlers are already registered for path '/api/v1/dict-items/:id'`。
agents 组后残留一段 dictItems.PUT/DELETE 重复块（存量引入，非当日改动），go build/go test 均
不报错——gin 的重复注册是**运行时 panic**，静态检查与单测都覆盖不到（单测不起完整路由或起的是
旧构造）。服务因此完全无法启动，但此前所有提交的门禁都是绿的。

## 规则

- 凡改动 `internal/server/router.go`（增删/挪动路由组、脚本化批量编辑），收尾必须**实际启动一次**
  （本地 `make dev` 或构建后运行，看到 `http server listening` 日志）再算完成；冒烟脚本要做启动探活。
- 对 router.go 做脚本化（python/sed）批量编辑时，前后各跑一次 `grep -c` 核对每个路由的注册次数。
- 相关修复提交：`fix(router): 移除 dict-items 的重复路由注册`。

## 关联

- aiDoc/plans/completed/2026-10-05-appauth-and-appuser-unification.md（本次冒烟发现的时机）
