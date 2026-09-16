# embodied-platform-service

<div align="center">

**embodied-platform 业务管理端**（设备/型号/租户/文件全部委外给平台，账号体系统一到平台）

Gin · GORM · Wire · Vue3 · TypeScript · Naive UI · 平台 SDK

</div>

---

## 架构定位

本系统是 [embodied-platform](../embodied-platform)（具身智能设备基础设施平台）的**业务管理端**，
按照平台《统一账号决策》（平台仓库 `aiDoc/notes/proposed/architecture/2026-09-12-unified-account-ego-login.md`）接入：

```
embodied-platform（平台 = 唯一身份源 + 设备/存储基础设施）
   ▲ ①登录/token 双用         ▲ ②商户 HMAC 开放面                ▲ ③管理面 JWT（可选）      ▲ ④storage-gateway API Key
   │ (浏览器直调)              │ /open-api/v1                      │ 仅「从平台同步用户」      │ :27091 文件存取
   │                          │ 设备域+租户域+型号域+物模型只读     │                          │
   │
┌─┴───────────────────────────┴───────────────────────────────────┴──────────────────────────┴───────────────────────┐
│ embodied-platform-service（本系统）                                                                                │
│ · 认证：平台 token + profile 自省（缓存 30-60s）+ 本地准入投影（开关+本地角色），无本地密码/JWT/会话                   │
│ · 保留：角色/权限(RBAC)、菜单、操作日志、手工 IP 黑名单、导出、租户(+开放面租户域同步)、应用用户(独立体系)             │
│ · 移除：商户模块、本地登录/会话/验证码/登录日志/在线用户、多云存储驱动（oss/cos/tos/minio）                            │
│ · 新增：设备管理（开放面代理）、型号管理（开放面型号域 + 物模型只读选择器）                                            │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

关键设计：

- **一个账户登录多个平台**：登录页由浏览器直调平台 `POST /api/v1/auth/login`（token 双用，既调本系统也直调平台）；
  本系统后端拿 token 调平台 `GET /auth/profile` 自省（平台侧吊销/改密在 30-60s 缓存 TTL 内感知）。
- **准入在业务平台侧**：本地 `platform_users` 投影表（`platform_user_id` 唯一 + 准入开关 + 本地角色），首登懒建、
  默认关闭；`platform.bootstrapAdmins` 内的平台账号首登自动放行并绑超管角色（冷启动引导）。
- **禁用/删除 = 同步吊销本地授权**：关闭准入/删除投影/调整角色时整体失效准入与 RBAC 决策缓存，
  该用户的存量平台 token 对本系统**立即** 403；平台侧账号不受影响（三层开关：平台禁用=全局、准入=项目×个人）。
- **租户与平台强一致同步**：本地租户创建/更新/删除平台先行、本地跟随（失败整体失败/回滚）。
  2026-09-16 起走平台开放面**租户域**（商户 HMAC，scope `tenant:*`）：创建时商户归属由平台服务端
  注入调用方（同步即自动进入本商户租户集，开放面设备注册随即覆盖）；存量租户可单条「同步至平台」补链。
- **型号管理走平台开放面型号域**（scope `model:*`）+ 物模型只读选择器（`thing-model:read`，
  版本仅 published）——与租户同步一样不再依赖管理面服务账号。
- **文件存储走平台 storage-gateway**：上传=后端中转（presign → PUT → complete）；下载=302 预签名 URL；
  平台存储未配置时自动降级本地磁盘（历史存量文件仍可读）。

## 部署前置清单（平台侧，对照平台 `docs/dev/integration-guide.md` §11）

1. 平台创建**商户**（`appKey`/`appSecret`）并配置 scopes：`device:*`、`telemetry:read`、`data-event:read`、
   `tenant:*`、`model:*`、`thing-model:read`（租户同步/型号管理/设备管理全部走商户 HMAC 单一凭证）；
2. （可选）`platform.admin` 管理账号：**仅**「从平台同步用户」便利功能使用（管理账号列表按平台治理
   决策不上开放面）；不配置时准入依赖首登懒建，其余功能不受影响；
3. （可选）平台管理端签发 **storage API Key**（`platform.storage`，目标桶读写 scope）；
4. 平台 CORS 白名单（`cors.allowedOrigins`）登记本系统前端 Origin（登录直调平台需要）；
5. NTP 校时（HMAC 时间戳偏差 ≤ 300s）。

以上凭证填入 `configs/config.yaml` 的 `platform` 段；启动时自动做连通性自检并打日志（失败不阻断启动）。

## 快速开始

```bash
# 前置：本机可达的 embodied-platform 主服务(27080)与 storage-gateway(27091)，并完成上面的清单
# 前端平台地址：web/.env.development 的 VITE_PLATFORM_API（默认 http://localhost:27080）
make web-install web-build run   # 打开 http://localhost:28180
# 用「平台侧的」管理员账号登录（默认引导账号 admin，见 platform.bootstrapAdmins）
```

开发热更新（前后端）：

```bash
make dev    # 后端 air 热加载 :28180 + 前端 Vite :28170（/api 代理到后端，登录/刷新直调平台）
```

## 平台 SDK 引入方式

`go.mod` 中联调期直接 replace 到同仓平台 SDK：

```go
require github.com/smilex/smilex-admin-gin/sdk v0.0.0
replace github.com/smilex/smilex-admin-gin/sdk => ../embodied-platform/sdk
```

正式部署经 GOPRIVATE 拉取（见 `embodied-platform/sdk/README.md`），删除 replace 即可。

## API（/api/v1，平台 token 认证）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /auth/profile | 本人信息（平台身份 + 准入状态 + 本地权限） |
| PUT | /auth/profile、/auth/password | 本人改资料/改密（以本人平台 token 代理到平台） |
| GET | /menus、/menus/search | 本人菜单树 / 菜单搜索 |
| GET/PUT/DELETE | /users、/users/:id/admission、/users/:id/roles、/users/sync | 准入管理（列表/开关/角色/移除/从平台同步） |
| GET/POST/PUT/DELETE | /roles、/permissions | 角色 / 菜单权限（本地 RBAC） |
| GET/POST/PUT/DELETE | /tenants、/tenants/:id/status、/tenants/:id/sync | 租户（与平台强一致同步 + 存量补链） |
| GET/POST | /devices、/devices/:id(/shadow//commands//telemetry//data-events)、/device-commands/:id | 设备域（开放面代理） |
| GET/POST/PUT/DELETE | /device-models、/thing-models、/thing-models/:id/versions | 型号管理 + 物模型只读选择器（管理面代理） |
| GET/POST/DELETE | /files、/files/:id/raw | 文件（平台存储；下载 302 预签名） |
| GET/POST/DELETE | /app-users、/app-auth/* | 应用用户（本系统独立 C 端体系，未接入平台） |
| GET/DELETE | /operation-logs、/ip-blacklist | 操作日志 / 手工 IP 黑名单 |

> 已知能力缺口（平台侧暂未提供）：设备更新/删除/状态流转（开放面与管理面均无删除）；
> 物模型管理、数据映射(TMP)、通信监控、OTA 列入后续规划。

## 目录结构

```
cmd/server/            入口 + wire 注入（启动含平台连通性自检）
internal/biz/          领域层：admission(准入)/auth(平台身份自省)/role/permission/log/file/export/
                       blacklist/tenant(平台同步)/device(开放面代理)/devmodel(管理面代理)/appuser
internal/data/         基础设施：GORM 仓储 + platform/(三类凭证客户端) + file/(platform/local 存储驱动)
internal/service/      应用层（薄用例）
internal/server/       传输层（Gin 路由/PlatformAuth 中间件/SPA 托管）
web/                   前端（Vue3 + TS + Naive UI；登录直调平台 web/src/api/platform.ts）
migrations/            三方言建表 SQL（菜单/权限种子在 AutoMigrate 中幂等补齐）
```

## 常用命令

```bash
make build       # 编译后端
make run         # 运行后端
make wire        # 重新生成 DI（改 Provider 后执行）
make test        # 测试
make web-dev     # 前端开发（热更新）
make web-build   # 前端构建（产物 web/dist，由后端静态托管）
```

## 旧数据说明

账号体系切换为「平台唯一身份源」后，本地 `users` / `user_roles` / `merchants` / `merchant_api_logs` /
`login_logs` 等旧表不再使用（AutoMigrate 已移除对应 PO）。存量库可手工清理：

```sql
DROP TABLE IF EXISTS merchant_api_logs, merchants, login_logs, user_roles, users;
```

历史本地文件（`files.driver='local'`）仍可读取；新上传一律走平台存储。

## License

MIT
