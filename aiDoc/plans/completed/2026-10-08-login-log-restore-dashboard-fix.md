<!-- last-updated: 2026-10-08 -->
# 管理端登录改后端代理 + 登录日志恢复 + 首页修复

## 目标

1. 管理端登录/刷新/登出/验证码由「浏览器直调平台」改为「本服务后端代理平台」，服务端自然记录登录日志（成功+失败、IP/UA），并恢复 LoginIPGuard 临时封禁与登录限流。
2. 恢复登录日志模块（曾于 3d1353fee 移除）：日志管理菜单下「登录日志」页，支持查询/清理/导出；保留期清理覆盖登录日志。
3. 修复首页：空白卡片填「今日登录次数」；活跃趋势与最近登录改用真实 login_logs 数据（不再用操作日志冒充）。

## 非目标

- 平台仓（embodied-platform）代码不改。
- 租户端 app-auth 登录链路不改（仍浏览器直调平台）。
- 登录日志不做跨实例分布式限流/封禁之外的安全增强。

## 假设

- 平台主服务公开 API（/api/v1/auth/login|refresh|logout|captcha）契约稳定，token 双用语义不变。
- 菜单/按钮种子（ensureSystemMenus/ensureSystemButtonPerms）幂等，存量库启动自动补齐 menu:loginLog 与 log:login:* 权限。
- 旧实现（3d1353fee^）可作为恢复底稿，但 exporter 等接口签名需对照现状适配。

## 影响面

- 后端：platform IdentityClient、biz/auth、service/auth、router/handler、wire 装配、biz+data+service log 上下文、data/model、data 种子、biz/export、migrations（三方言 DDL）、dashboard 上下文、oplog 动作映射。
- 前端：api（index/platform/request 复核）、stores/user、Login.vue、新增 LoginLogs.vue + loginLog locales、router/dynamic.ts、Dashboard.vue + dashboard locales。
- 契约：本服务新增公开端点 POST /api/v1/auth/login|refresh、GET /auth/captcha；basic 组新增 POST /auth/logout；新增 /api/v1/login-logs（GET/DELETE/POST export）；dashboard stats 结构加 cards.today_login_count。

## 验收标准

- [ ] `go build ./...` 与 `go vet ./...` 通过（wire_gen.go 已同步）
- [ ] 前端 typecheck/build 通过
- [ ] 错误密码登录 → 401 + login_logs 落失败记录（IP/UA/msg 正确）；连续失败触发临时封禁
- [ ] 正确密码登录 → 200 + 成功记录；登录日志页可查询/清理/导出
- [ ] 首页三张卡有数（含今日登录次数）、趋势图登录线与操作线口径分离、最近登录为真实登录记录

## 工作项

| # | 工作项 | owner | 依赖 | 建议写范围 | 验证命令 | 状态 |
|---|---|---|---|---|---|---|
| 1 | 登录代理 + 登录日志模块 + dashboard 后端 | coder 子代理 | 无 | internal/、cmd/server、migrations/ | `go build ./... && go vet ./...` | 进行中 |
| 2 | 前端登录改造 + 登录日志页 + 首页修复 | coder 子代理 | 接口契约已在任务书固化 | web/src/ | `npm run typecheck`（web/） | 进行中 |
| 3 | 集成联调与真实入口冒烟 | 主 agent | 1、2 | — |  curl 冒烟 + 浏览器流程 | 未开始 |
| 4 | 文档留痕与契约同步、漂移自检 | 主 agent | 1、2 | aiDoc/、AGENTS.md、../AGENTS.md | check_sync.py 或路由表核对 | 进行中（计划/记忆先行） |

## 验证命令

```bash
go build ./... && go vet ./...
cd web && npm run typecheck   # 以 package.json scripts 实际命令为准
# 冒烟（需平台 :27080 可达）：
curl -X POST localhost:28180/api/v1/auth/login -d '{"username":"x","password":"wrong"}'  # 401 + 失败日志
curl -X POST localhost:28180/api/v1/auth/login -d '{"username":"<admin>","password":"<ok>"}' # 200 + 成功日志
curl -H "Authorization: Bearer <token>" "localhost:28180/api/v1/login-logs?page=1&page_size=10"
```

## 回滚 / 迁移

- login_logs 表由 AutoMigrate 自动创建（migrations 三方言 DDL 为参考口径）；回滚 =  revert 代码 + drop table login_logs（无存量数据迁移）。
- 前端回滚即恢复 platform.ts 直调函数（git revert 本变更）。

## 当前状态

- 已完成并归档（2026-10-08）。

## 交付摘要（完成时填写）

全部工作项完成：

- **后端**（`go build`/`go vet`/`go test ./internal/... ./pkg/...` 均 passed）：IdentityClient 新增 Login/Refresh/Logout/Captcha 代理（公开路由 + LoginIPGuard + IP 限流 10/min）；登录日志模块按 3d1353fee^ 底稿恢复并适配（双类型异步队列、login_logs 表 AutoMigrate + 三方言 DDL、menu:loginLog 与 log:login:* 种子、login_log 导出器适配现行 NameKey/i18n 接口、保留期清理覆盖）；dashboard 新增 today_login_count、LoginTrend/RecentLogins 改查 login_logs 真实口径。关键坑：旧种子 code 在 `obsoletePermCodes` 清单里，不移除会被 migrateLegacy 每次启动删掉。
- **前端**（`vue-tsc -b` 与 eslint passed）：auth 函数收回 axios 实例（命中 isAuthEndpoint 排除正则）；LoginLogs.vue 与 loginLog locales 恢复并校准现规范；首页空卡填「今日登录次数」，趋势/最近登录标签口径修正。
- **真实入口冒烟**（浏览器 + 平台 27080 在线）：登录页登录成功跳转首页；四卡含今日登录次数；趋势图登录/成功/操作三线分离；最近登录为真实流水；登录日志页查询/展示正常。期间同 IP 限流（10/min）被真实触发并正确提示——功能符合设计，开发期多浏览器共享 ::1 会互相挤占配额。
- **文档**：决策记录落 `notes/implemented/architecture/2026-10-08-admin-login-backend-proxy.md`；boundary.md、frontend-rules.md、architecture-rules.md、repo-profile.md、development-workflow.md、system-map.md、api-example.md、根 AGENTS.md、configs/config.yaml 注释、项目集 ../AGENTS.md 均已同步；平台仓旧提案 2026-09-12 就地标注被部分取代。
- 偏差：无实质偏差；logout 后旧 token 在自省缓存 TTL（30–60s）内仍可用为既有文档化权衡，非本次引入。
