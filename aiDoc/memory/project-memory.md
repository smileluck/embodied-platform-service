<!-- last-updated: 2026-10-08 -->
# 项目记忆索引

## 长期记忆

暂无。

## 业务需求记忆

- [business/2026-10-08-租户用户体系.md](business/2026-10-08-租户用户体系.md) — 开放租户端给企业租户：AppUser 身份复用 + 本地租户级双角色授权 + /app-api/v1 租户面

## 经验记忆（lessons）

- [lessons/2026-10-08-wire-bind-vs-interface-provider.md](lessons/2026-10-08-wire-bind-vs-interface-provider.md) — wire Provider 直接返回 biz 接口时不需要（也不能）再 wire.Bind
- [lessons/2026-10-08-frontend-route-namespace-collision.md](lessons/2026-10-08-frontend-route-namespace-collision.md) — 新增前端路由区先查既有菜单路径；守卫分流勿用裸前缀 startsWith
- [lessons/2026-10-08-httpserver-constructor-missing-field.md](lessons/2026-10-08-httpserver-constructor-missing-field.md) — HTTPServer 构造体漏字段赋值零告警，整域 handler nil panic→空体 500

## 维护说明

- 新增记忆时创建文件并更新此索引
- 过时记忆及时清理，并同步清理索引条目
- 索引每条一行：文件相对路径 + 一句话摘要
