<!-- last-updated: 2026-10-09 -->
# 项目记忆索引

## 长期记忆

暂无。

## 业务需求记忆

- [business/2026-10-08-租户用户体系.md](business/2026-10-08-租户用户体系.md) — 开放租户端给企业租户：AppUser 身份复用 + 本地租户级双角色授权 + /app-api/v1 租户面
- [business/2026-10-08-型号商户归属收敛接入.md](business/2026-10-08-型号商户归属收敛接入.md) — 设备相关模块对齐平台型号商户归属收敛：merchant_id 全链路透出 + DeleteModel 404 不再幂等放行 + 型号页归属列与通用型号只读
- [business/2026-10-09-物模型与数据映射页面.md](business/2026-10-09-物模型与数据映射页面.md) — 设备中心新增物模型只读页 + 数据映射完整管理页（平台开放面新增 mapping 域含写）+ 权限码；含设备列表可见性收口
- [business/2026-10-09-商户自定义物模型.md](business/2026-10-09-商户自定义物模型.md) — 物模型页升级为可管理：平台开放面放写端点（scope thing-model:write），写仅本商户、通用只读、读=通用+本商户
- [business/2026-10-09-租户权限码目录动态化.md](business/2026-10-09-租户权限码目录动态化.md) — 门户权限码事实源迁到商户端注册表：启动自动同步平台（PUT /tenant-user-perms，按商户隔离、只增不删），平台目录动态化后新增权限码免平台发版
- [business/2026-10-08-登录日志恢复与首页修复.md](business/2026-10-08-登录日志恢复与首页修复.md) — 管理端登录改后端代理平台，恢复登录日志模块；首页空卡片与活跃趋势口径修复
- [business/2026-10-08-主题恢复工业琥珀配色.md](business/2026-10-08-主题恢复工业琥珀配色.md) — 全站主题从清水蓝恢复石墨深壳 + 工业琥珀色板，保留亮暗双套并修复暗色可读性

## 经验记忆（lessons）

- [lessons/2026-10-08-wire-bind-vs-interface-provider.md](lessons/2026-10-08-wire-bind-vs-interface-provider.md) — wire Provider 直接返回 biz 接口时不需要（也不能）再 wire.Bind
- [lessons/2026-10-08-frontend-route-namespace-collision.md](lessons/2026-10-08-frontend-route-namespace-collision.md) — 新增前端路由区先查既有菜单路径；守卫分流勿用裸前缀 startsWith
- [lessons/2026-10-08-httpserver-constructor-missing-field.md](lessons/2026-10-08-httpserver-constructor-missing-field.md) — HTTPServer 构造体漏字段赋值零告警，整域 handler nil panic→空体 500
- [lessons/2026-10-08-nselect-query-null-placeholder.md](lessons/2026-10-08-nselect-query-null-placeholder.md) — 搜索区 n-select 的 query 字段必须初始化 null；'' 会被当作已选值导致 placeholder 不显示（已晋升至 frontend-rules.md 组件规范）
- [lessons/2026-10-09-cross-repo-parallel-session-grep-misjudge.md](lessons/2026-10-09-cross-repo-parallel-session-grep-misjudge.md) — 跨仓并行会话中判「文件缺失/断头」前先绝对路径重 grep + mtime + 复跑 build；cwd 逐命令重置与 ugrep 静默 warning 会造成空结果误判
- [lessons/2026-10-08-obsolete-perm-codes-restore.md](lessons/2026-10-08-obsolete-perm-codes-restore.md) — 恢复旧菜单/权限 code 前必须先移出 obsoletePermCodes 淘汰清单，否则种子被 migrateLegacy 反复清掉
- [lessons/2026-10-09-vue-i18n-literal-braces.md](lessons/2026-10-09-vue-i18n-literal-braces.md) — locale 文案要展示字面 `{xxx}`（如 source_topic 占位符）时，调用处必须传同名字面参数，否则 vue-i18n 当插值变量解析
- [lessons/2026-10-09-menu-seed-softdelete-restore.md](lessons/2026-10-09-menu-seed-softdelete-restore.md) — 种子「Unscoped 判存在 + 缺失插入」必须配软删恢复分支；菜单软删会引发按钮孤儿化次生症状
- [lessons/2026-10-09-debug-server-port-occupancy.md](lessons/2026-10-09-debug-server-port-occupancy.md) — 调试二进制占用 dev 端口会让 air 热重载静默失效（bind 失败只进日志、build text-file-busy 致 air wedge）；「改了不生效」先 lsof 确认监听者

## 维护说明

- 新增记忆时创建文件并更新此索引
- 过时记忆及时清理，并同步清理索引条目
- 索引每条一行：文件相对路径 + 一句话摘要

- [lessons/2026-10-09-new-table-legacy-name-collision.md](lessons/2026-10-09-new-table-legacy-name-collision.md) — 新增本地表先查 migrateLegacy 清理名单，撞旧表名会被每次启动反复清空（tenant_user_roles→tenant_user_role_binds）
