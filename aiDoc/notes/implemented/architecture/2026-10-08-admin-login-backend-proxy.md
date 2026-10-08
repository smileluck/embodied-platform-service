<!-- last-updated: 2026-10-08 -->
# 管理端登录改为本服务后端代理平台（登录日志恢复的前提）

## 问题

管理端登录此前是浏览器直调平台 `/auth/login`（token 双用），本服务后端完全不经过登录请求。用户要求在本服务「日志管理」下恢复登录日志（含失败记录、IP/UA），而直调架构下服务端无法观测登录成败——失败发生在浏览器与平台之间，本服务无从记录。

## 提案 / 决策

管理端登录/刷新/登出/验证码改为经本服务后端代理平台：本服务新增公开端点 `POST /api/v1/auth/login`、`POST /api/v1/auth/refresh`、`GET /api/v1/auth/captcha` 与认证端点 `POST /api/v1/auth/logout`，后端 `IdentityClient` 以普通 HTTP 调平台同名公开 API（非开放面、无 HMAC）。token 仍由平台签发、双用于两侧。

由此获得：

- 登录成功/失败由服务端自然落 `login_logs`（含平台返回的失败原因、IP、UA）；
- 恢复挂载 `LoginIPGuard`（连续失败临时封禁，Redis 计数）与登录 IP 限流；
- 管理端不再依赖平台 CORS 白名单（租户端 app-auth 仍直调，白名单仍需保留）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 保持直调 + 前端上报登录结果到本服务公开接口 | 数据可伪造（旁路审计数据可信度差）；用户明确倾向于代理架构（「不应该调本服务再转发平台吗」） |
| 后端 PlatformAuth 中间件按「新 token 首次出现」推断登录 | 拿不到失败记录；token 刷新会被误记为新登录，口径失真 |
| 经平台开放面查询平台侧登录日志 | 平台开放面无登录日志接口，需跨仓先平台后 service 的契约变更，成本高；且用户要求本服务「独立记录」 |

## 验收标准

- [x] 登录页错误密码 → 401 透传平台本地化 msg，且 `login_logs` 落失败记录（IP/UA/msg 正确）
- [x] 登录 IP 限流生效（同 IP 10 次/分，429 + 「登录尝试过于频繁」）；LoginIPGuard 成功清零失败计数已验证（Redis 计数键删除）；临时封禁的 403 触发面复用 app-auth 既有中间件路径，未单独构造连续失败场景验证
- [x] 正确密码 → 200 返回 token，前端登录→首页→登录日志页真实冒烟通过
- [x] 管理端页面功能不依赖浏览器直连平台（仅租户端 app-auth 例外）

## 风险与后果

- 平台账号密码经过本服务内存转发：不落盘、不进操作日志（/auth/login 挂公开组无 OpLog 审计）；需在评审中确认无日志泄露面。
- 本服务可用性成为登录链路前提：本服务宕机则管理端无法登录（此前平台在即可登录）。
- 跨项目文档同步义务：`../AGENTS.md`（项目集）「管理端认证」行、本仓 `aiDoc/contracts/boundary.md`、根 `AGENTS.md` 契约节均需在同一变更内更新。

## 交叉链接

- 计划：`aiDoc/plans/completed/2026-10-08-login-log-restore-dashboard-fix.md`
- 被部分推翻的旧提案：平台仓 `embodied-platform/aiDoc/notes/proposed/architecture/2026-09-12-unified-account-ego-login.md` 第 2 点（已就地标注）
- 相关代码：`internal/data/platform/identity.go`（IdentityClient 代理方法）、`internal/server/router.go`（公开 auth 路由 + LoginIPGuard）、`internal/server/handler_auth.go`
- 背景提交：3d1353fee（重构移除登录日志、改浏览器直调平台）
