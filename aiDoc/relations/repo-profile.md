<!-- last-updated: 2026-10-03 -->
# 项目档案（repo profile）

> 项目定位与技术栈速查。所有内容必须来自真实配置文件与代码探测，禁止编造。

## 项目定位

**SmileX Admin（github.com/smilex/smilex-admin-gin）**：embodied 平台的商户侧**业务管理端**（mixed：Go 后端 + Vue 3 前端同仓）。定位是商户日常运营入口——成员/角色/权限、字典、文件、通知、任务、监控、智能体/MCP/技能、设备与租户管理；平台（embodied-platform，见项目集根 `../../AGENTS.md`）是唯一身份源，管理端登录由本服务代理转发平台（不落密码），本服务做 token 自省 + 本地准入投影 + 登录日志。

## 核心技术栈

| 组件 | 路径 | 技术栈 |
|---|---|---|
| web-backend | （根） | Go 1.26 + Gin + GORM（mysql/postgres/sqlite 三驱动）+ wire（DI）+ viper + zap + go-redis + robfig/cron |
| web-frontend | `web/` | Vue 3 + TypeScript + Vite 5 |

关键库：JWT（`github.com/golang-jwt/jwt/v5`）、系统监控（`shirou/gopsutil/v4`）、测试 Redis（`alicebob/miniredis/v2`）。

## 前端技术栈

| 类别 | 选型 |
|---|---|
| 框架 | Vue 3（`<script setup>` + TS） |
| 构建工具 | Vite 5（dev 端口 28170，`/api` 代理到 28180） |
| UI 库 | Naive UI（+ `@vicons/ionicons5`） |
| 状态管理 | Pinia（`web/src/stores/`） |
| 路由 | vue-router 4（登录后按菜单动态生成，`web/src/router/dynamic.ts`） |
| 样式方案 | 组件库主题 + `web/src/styles/`（暗壳亮芯 + 工业琥珀主题，含 sx-led/sx-plate 签名元素） |
| 国际化 | vue-i18n（`web/src/locales/{zh-CN,en-US}`） |
| 可视化 | echarts |

## 包管理

- Go：go mod（依据：`go.mod`）
- 前端：npm（依据：`web/package-lock.json`）

## 核心特性

| 特性 | 说明 | 证据 |
|---|---|---|
| 统一响应信封 | `{code, msg, data}`，code 0=成功；错误消息按 Accept-Language 本地化 | `pkg/response/response.go:Body` |
| 双认证体系 | 平台身份（管理端，token 自省+本地准入）/ 本地应用用户（AppJWT，`/app-auth`） | `internal/server/middleware/middleware.go:PlatformAuth`、同文件 `AppJWT` |
| RBAC 二级缓存 | 权限判定 L1 进程内存 + L2 Redis，变更时整体失效 | `internal/server/middleware/middleware.go:RBAC`、`pkg/cache` |
| 平台开放面接入 | 商户 HMAC 签名调用平台 open-api；storage-gateway 托管文件 | `internal/platformsdk/client.go:Client`、`internal/data/platform/` |
| 软删 + 唯一槽位释放 | 软删行归档唯一列，允许同编码重建 | `internal/data/archive_unique.go:ArchiveUniqueColumns` |
| 多数据库 | 同一套 GORM 模型支持 mysql/postgres/sqlite，DDL 三方言 | `internal/data/data.go:NewData`、`migrations/` |
| 热重载开发 | air（后端）+ Vite（前端）一键 `make dev` | `Makefile:dev`、`.air.toml` |
| 前端静态托管 | 生产模式 `web/dist` 由后端直接托管 | `internal/server/router.go:registerStatic`、`Makefile:web-build` |
