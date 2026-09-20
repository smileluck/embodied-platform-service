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
   ▲ ①登录/token 双用                                  ▲ ②商户 HMAC 开放面                     ▲ ③storage-gateway
   │ (浏览器直调；后端持同一                           │ /open-api/v1                          │ API Key 文件存取
   │   token 自省/代理资料)                            │ 设备域+租户域+型号域+物模型只读       │ (:27091)
   │                                                   │ +用户域(成员/准入)                    │
┌──┴───────────────────────────────────────────────────┴───────────────────────────────────────┴─────────────────┐
│ embodied-platform-service（本系统）                                                                            │
│ · 认证：平台 token + profile 自省（缓存 30-60s）+ 本地准入投影（开关+本地角色），无本地密码/JWT/会话           │
│ · 保留：角色/权限(RBAC)、菜单、操作日志、手工 IP 黑名单、导出、租户(+开放面租户域同步)、应用用户(独立体系)     │
│ · 移除：商户模块、本地登录/会话/验证码/登录日志/在线用户、多云存储驱动(oss/cos/tos/minio)、平台管理面通道      │
│ · 新增：设备管理（开放面代理）、型号管理（开放面型号域 + 物模型只读选择器）                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

关键设计：

- **一个账户登录多个平台**：登录页由浏览器直调平台 `POST /api/v1/auth/login`（token 双用，既调本系统也直调平台）；
  本系统后端拿 token 调平台 `GET /auth/profile` 自省（平台侧吊销/改密在 30-60s 缓存 TTL 内感知）。
- **准入在业务平台侧**：本地 `platform_users` 投影表（`platform_user_id` 唯一 + 准入开关 + 本地角色）。
  准入唯一事实源=平台商户成员绑定：成员经「用户同步/新增成员」建投影；平台标记的商户管理员首登自动准入；
  平台侧解绑本商户（含管理台移除准入接入方）后 30-60s 内本地同步拒绝（随身份自省缓存 TTL）。
  本系统无本地超管/引导账号（bootstrapAdmins 机制已移除）。
- **禁用/删除 = 同步吊销本地授权**：关闭准入/删除投影/调整角色时整体失效准入与 RBAC 决策缓存，
  该用户的存量平台 token 对本系统**立即** 403；平台侧账号不受影响（三层开关：平台禁用=全局、准入=项目×个人）。
- **租户与平台强一致同步**：本地租户创建/更新/删除平台先行、本地跟随（失败整体失败/回滚）。
  2026-09-16 起走平台开放面**租户域**（商户 HMAC，scope `tenant:*`）：创建时商户归属由平台服务端
  注入调用方（同步即自动进入本商户租户集，开放面设备注册随即覆盖）；存量租户可单条「同步至平台」补链。
  租户 `code` 唯一性=**商户内**（2026-09-18 起平台侧收敛为商户内复合唯一）：他商户同 code 不构成冲突。
  本地删除为软删，且删时改写 `name`/`code`（后缀 `#del#<id>`）释放单列唯一索引槽位，
  软删后同 code/name 重建不会误报重复（避免误触发 Create 回滚平台侧的连锁）。
  删除治理（2026-09-18）：商户绑定租户的删除入口=本系统（平台管理面已拒删；平台侧存在关联应用用户
  或设备时 409 透传）；若平台侧租户意外丢失（404），更新/启停自动按同 code 补链重建并回填新平台 ID，
  删除按「已删」幂等放行——本地租户不会再被平台侧 404 卡死。
- **数据完整性收口（2026-09-19）**：设备注册遇平台 403（本地租户镜像悬挂）自动补链一次重试；
  平台账号被删后的孤儿准入投影在「同步成员」时自动回收（含角色绑定，不再阻塞角色删除）；
  文件/导出下载遇平台对象已清理（404）返回「文件已失效」而非 500；型号删除平台 404 幂等；
  本地 app_users/permissions 删除墓碑释放唯一槽位（删后重建同值不误报）、IP 解封后重封复活原行。
- **型号管理走平台开放面型号域**（scope `model:*`）+ 物模型只读选择器（`thing-model:read`，
  版本仅 published）——与租户同步一样不再依赖管理面服务账号。
- **文件存储走平台 storage-gateway**：上传=后端中转（presign → PUT → complete）；下载=302 预签名 URL；
  平台存储未配置时自动降级本地磁盘（历史存量文件仍可读）。

## 部署前置清单（平台侧，对照平台 `docs/dev/integration-guide.md` §11）

1. 平台创建**商户**（`appKey`/`appSecret`）并配置 scopes：`device:*`、`telemetry:read`、`data-event:read`、
   `tenant:*`、`model:*`、`thing-model:read`、`user:*`（租户同步/型号管理/设备管理/成员管理全部走商户 HMAC 单一凭证）；
2. （可选）平台管理端签发 **storage API Key**（`platform.storage`，目标桶读写 scope）；
3. 平台 CORS 白名单（`cors.allowedOrigins`）登记本系统前端 Origin（登录直调平台需要）；
4. NTP 校时（HMAC 时间戳偏差 ≤ 300s）。

以上凭证填入 `configs/config.yaml` 的 `platform` 段；启动时自动做连通性自检并打日志（失败不阻断启动）。
注：用户（成员）管理走开放面 `user:*`——列表=平台本商户绑定成员，新增=平台无此账号则创建（初始密码在本系统
添加用户时设置）有则绑定，删除=解除关联（平台账号保留）；`platform.admin` 管理面通道已移除，无需配置。

**商户管理员**：在平台管理端「商户管理 → 成员」里给成员开管理员标记（平台是唯一事实源）。被标记的账号
首次请求本系统即自动准入并绑定内置「商户管理员」角色（ID=2，锁定）；平台撤标记后 30-60s 内自动回收
（随身份自省缓存 TTL）。本系统按开放面 `/ping` 自动识别自己的商户身份，无需额外配置；
**升级顺序须平台先于本系统**（旧版平台 profile 不含商户标记字段，功能不生效但无害）。

**准入状态（2026-09-18）**：本端「用户准入」页开/关准入 = 平台先行写回 `merchant_users.admitted`
（开放面 `PUT /users/:id/admission`，scope `user:setAdmission`；已授 `user:*` 域通配的商户自动覆盖），
本地投影跟随；平台侧成员列表同步显示「业务端准入」状态。用户列表与准入判定均以平台为事实源。

## 快速开始

```bash
# 前置：本机可达的 embodied-platform 主服务(27080)与 storage-gateway(27091)，并完成上面的清单
# 前端平台地址：web/.env.development 的 VITE_PLATFORM_API（默认 http://localhost:27080）
make web-install web-build run   # 打开 http://localhost:28180
# 登录账号=平台侧标记的「商户管理员」（平台商户管理 → 成员 → 开管理员标记，首登自动准入）
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

Windows 环境说明：

- 安装 make：`choco install make` 或 `scoop install make`（MinGW 用户可直接用 `mingw32-make`），CMD / PowerShell / Git Bash 均可运行
- 所有 make 目标均已适配：`make build` 产物为 `bin\server.exe`，`make dev` 自动使用 `.air.windows.toml`
- `make dev` 在同一控制台并行启动后端 air 与前端 Vite，Ctrl+C 一并停止（与 macOS/Linux 行为一致）

## 🐳 Docker 部署（docker compose）

一条命令拉起完整服务（app + MySQL + Redis），首次启动自动建表 + 种子数据：

```bash
cp .env.example .env          # 并填写 MYSQL 密码与 JWT_SECRET
docker compose up -d          # 构建镜像并启动
# 打开 http://localhost:8080
```

说明：

- **多阶段构建**：Node 构建前端 dist → Go 编译单二进制（含 CGO，三种数据库驱动均可用）→ Alpine 运行时，镜像内已含 SPA 托管，无需 Nginx
- **配置注入**：所有配置项支持 `APP_` 前缀环境变量覆盖（`.` → `_`，如 `APP_DB_MYSQL_HOST` 覆盖 `db.mysql.host`），优先级高于镜像内置的 config.yaml；改完 `docker compose up -d` 重建即生效
- **数据持久化**：named volume——`app-data`（上传/导出/sqlite）、`app-logs`（应用日志）、`mysql-data`、`redis-data`
- **换数据库**：`.env` 里 `DB_DRIVER=postgres` 并放开 docker-compose.yml 中的 postgres 服务；`DB_DRIVER=sqlite` 则零外部依赖（数据落 `app-data` 卷）
- **默认生产语义**：release 模式、登录验证码开启、日志同时输出控制台（`docker logs`）
- **端口冲突**：宿主机 8080 已被占用（如本地开发后端）时，在 `.env` 中改 `APP_PORT` 换对外端口
- 常用命令：`make docker-up` / `make docker-down` / `make docker-logs` / `make docker-rebuild`
- **平台 SDK 依赖**：本服务依赖 `github.com/smilex/smilex-admin-gin/sdk`（联调期 replace 到本地 `../embodied-platform/sdk`）；镜像构建前需先移除 replace，改走 GOPRIVATE 拉取

## API（/api/v1，平台 token 认证）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /auth/profile | 本人信息（平台身份 + 准入状态 + 本地权限） |
| PUT | /auth/profile、/auth/password | 本人改资料/改密（以本人平台 token 代理到平台） |
| GET | /menus、/menus/search | 本人菜单树 / 菜单搜索 |
| GET/POST/PUT/DELETE | /users、/users/:id/admission、/users/:id/roles、/users/sync | 准入管理（列表/新增/开关/角色/移除/从平台同步） |
| GET/POST/PUT/DELETE | /roles、/permissions | 角色 / 菜单权限（本地 RBAC） |
| GET/POST/PUT/DELETE | /tenants、/tenants/:id/status、/tenants/:id/sync | 租户（与平台强一致同步 + 存量补链） |
| GET/POST | /devices、/devices/:id(/shadow//commands//telemetry//data-events)、/device-commands/:id | 设备域（开放面代理） |
| GET/POST/PUT/DELETE | /device-models、/thing-models、/thing-models/:id/versions | 型号管理（开放面型号域）+ 物模型只读选择器 |
| GET/POST/DELETE | /files、/files/:id/raw | 文件（平台存储；下载 302 预签名） |
| GET/POST/DELETE | /app-users、/app-auth/* | 应用用户（本系统独立 C 端体系，未接入平台） |
| GET/DELETE | /operation-logs、/ip-blacklist | 操作日志 / 手工 IP 黑名单 |

> 已知能力缺口（平台侧暂未提供）：设备更新/删除/状态流转（开放面与管理面均无删除）；
> 物模型管理、数据映射(TMP)、通信监控、OTA 列入后续规划。

## 目录结构

```
cmd/server/            入口 + wire 注入（启动含平台连通性自检）
internal/biz/          领域层：admission(准入)/auth(平台身份自省)/role/permission/log/file/export/
                       blacklist/tenant(平台同步)/device(开放面代理)/devmodel(开放面代理)/appuser
internal/data/         基础设施：GORM 仓储 + platform/(商户 HMAC 开放面/用户 token 自省/storage 客户端) + file/(platform/local 存储驱动)
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
make docker-up   # Docker 构建并启动完整服务（app + MySQL + Redis）
make docker-down # 停止并移除容器
```

## 旧数据说明

账号体系切换为「平台唯一身份源」后，本地 `users` / `user_roles` / `merchants` / `merchant_api_logs` /
`login_logs` 等旧表不再使用（AutoMigrate 已移除对应 PO）。存量库可手工清理：

```sql
DROP TABLE IF EXISTS merchant_api_logs, merchants, login_logs, user_roles, users;
```

历史本地文件（`files.driver='local'`）仍可读取；新上传一律走平台存储。

密钥与敏感配置：

- 仓库不保存任何可用密钥：`jwt.secret` 默认为空。
- 本地开发（debug 模式）secret 留空可正常启动（仅告警）；**release 模式下空或长度不足 32 位的
  jwt secret 会拒绝启动**，须以环境变量 `APP_JWT_SECRET` 注入强随机值（`openssl rand -base64 32`）。
- 更换 jwt secret 后所有已签发 token 失效需重新登录；若未单独配置 `agent.cryptoKey`，
  已保存的供应商 API Key 需重新保存一次。
- 跨域访问 API 需配置 `server.corsOrigins` 白名单（默认同源模式，不下发 CORS 头）。

## License

MIT
