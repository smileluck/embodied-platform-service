<!-- last-updated: 2026-10-08 -->
# 项目记忆索引

## 长期记忆

暂无。

## 业务需求记忆

- [business/2026-10-08-租户用户体系.md](business/2026-10-08-租户用户体系.md) — 开放租户端给企业租户：AppUser 身份复用 + 本地租户级双角色授权 + /app-api/v1 租户面
- [business/2026-10-08-登录日志恢复与首页修复.md](business/2026-10-08-登录日志恢复与首页修复.md) — 管理端登录改后端代理平台，恢复登录日志模块；首页空卡片与活跃趋势口径修复
- [business/2026-10-08-主题恢复工业琥珀配色.md](business/2026-10-08-主题恢复工业琥珀配色.md) — 全站主题从清水蓝恢复石墨深壳 + 工业琥珀色板，保留亮暗双套并修复暗色可读性

## 经验记忆（lessons）

- [lessons/2026-10-08-wire-bind-vs-interface-provider.md](lessons/2026-10-08-wire-bind-vs-interface-provider.md) — wire Provider 直接返回 biz 接口时不需要（也不能）再 wire.Bind
- [lessons/2026-10-08-frontend-route-namespace-collision.md](lessons/2026-10-08-frontend-route-namespace-collision.md) — 新增前端路由区先查既有菜单路径；守卫分流勿用裸前缀 startsWith
- [lessons/2026-10-08-httpserver-constructor-missing-field.md](lessons/2026-10-08-httpserver-constructor-missing-field.md) — HTTPServer 构造体漏字段赋值零告警，整域 handler nil panic→空体 500
- [lessons/2026-10-08-nselect-query-null-placeholder.md](lessons/2026-10-08-nselect-query-null-placeholder.md) — 搜索区 n-select 的 query 字段必须初始化 null；'' 会被当作已选值导致 placeholder 不显示（已晋升至 frontend-rules.md 组件规范）
- [lessons/2026-10-08-obsolete-perm-codes-restore.md](lessons/2026-10-08-obsolete-perm-codes-restore.md) — 恢复旧菜单/权限 code 前必须先移出 obsoletePermCodes 淘汰清单，否则种子被 migrateLegacy 反复清掉

## 维护说明

- 新增记忆时创建文件并更新此索引
- 过时记忆及时清理，并同步清理索引条目
- 索引每条一行：文件相对路径 + 一句话摘要
