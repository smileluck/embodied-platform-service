<!-- last-updated: 2026-10-19 -->
# 项目记忆索引

## 长期记忆

暂无。

## 业务需求记忆

- [business/2026-10-08-租户用户体系.md](business/2026-10-08-租户用户体系.md) — 开放租户端给企业租户：平台第四套身份 tenant_users（单租户绑定）+ /tenant-api/v1 租户门户（成员自治 + 设备只读）
- [business/2026-10-08-型号商户归属收敛接入.md](business/2026-10-08-型号商户归属收敛接入.md) — 设备相关模块对齐平台型号商户归属收敛：merchant_id 全链路透出 + DeleteModel 404 不再幂等放行 + 型号页归属列与通用型号只读
- [business/2026-10-09-物模型与数据映射页面.md](business/2026-10-09-物模型与数据映射页面.md) — 设备中心新增物模型只读页 + 数据映射完整管理页（平台开放面新增 mapping 域含写）+ 权限码；含设备列表可见性收口
- [business/2026-10-09-商户自定义物模型.md](business/2026-10-09-商户自定义物模型.md) — 物模型页升级为可管理：平台开放面放写端点（scope thing-model:write），写仅本商户、通用只读、读=通用+本商户
- [business/2026-10-09-租户权限码目录动态化.md](business/2026-10-09-租户权限码目录动态化.md) — 门户权限码事实源迁到商户端注册表：启动自动同步平台（PUT /tenant-user-perms，按商户隔离、只增不删），平台目录动态化后新增权限码免平台发版
- [business/2026-10-09-租户门户日志.md](business/2026-10-09-租户门户日志.md) — 门户登录/操作审计独立两表（tenant_login_logs/tenant_operation_logs 带 tenant_id）+ 门户日志页（log:list）+ 管理端可查可清
- [business/2026-10-08-登录日志恢复与首页修复.md](business/2026-10-08-登录日志恢复与首页修复.md) — 管理端登录改后端代理平台，恢复登录日志模块；首页空卡片与活跃趋势口径修复
- [business/2026-10-08-主题恢复工业琥珀配色.md](business/2026-10-08-主题恢复工业琥珀配色.md) — 全站主题从清水蓝恢复石墨深壳 + 工业琥珀色板，保留亮暗双套并修复暗色可读性
- [business/2026-10-10-四项界面缺陷修复批次.md](business/2026-10-10-四项界面缺陷修复批次.md) — 通知按用户发布选项预载 + 设备页状态 i18n/时间统一格式 + 定时任务禁用不被执行收尾复活（手动执行保持放行）
- [business/2026-10-10-租户部门管理.md](business/2026-10-10-租户部门管理.md) — 租户内部数据隔离基础能力：本地部门树（tenant_depts）+ 用户多部门归属（tenant_user_dept_binds）+ 管理端部门页/分配/筛选；设备级隔离留二期
- [business/2026-10-11-设备中心型号与状态显示修复.md](business/2026-10-11-设备中心型号与状态显示修复.md) — 设备中心型号列/详情按名称显示（前端映射，兜底 #id）；状态 i18n 复查为外部旧 dev server 占用 28170 所致
- [business/2026-10-11-命令下发读取物模型服务.md](business/2026-10-11-命令下发读取物模型服务.md) — 设备详情命令下发类型下拉从硬编码急停改为：型号绑定物模型版本（pin）解析 services 生成 + 内置急停兜底 + tag 手输
- [business/2026-10-12-全站时间格式统一.md](business/2026-10-12-全站时间格式统一.md) — 全站时间展示统一 yyyy-MM-dd HH:mm:ss：前端 12 文件收敛 formatDateTime + 后端 pkg/timex 单源；机器字段（RFC3339/unix 秒/图表轴）例外不动
- [business/2026-10-12-设备列表绑定租户.md](business/2026-10-12-设备列表绑定租户.md) — 设备改绑租户：平台开放面 PUT /devices/:id/tenant（scope device:bind-tenant）+ 本仓代理链（RBAC device:bind）+ 设备列表操作列改绑弹窗；同日补充设备列表「租户」列名称映射显示
- [business/2026-10-12-登录页默认用户名.md](business/2026-10-12-登录页默认用户名.md) — 登录页用户名不再默认填 admin；验证发现 28170/28180 被旧目录副本前后端口双占（本仓验证走独立端口 38180）
- [business/2026-10-16-成员准入定时对账.md](business/2026-10-16-成员准入定时对账.md) — admission_reconcile 定时对账（*/5）：平台准入/解绑/管理员标记变更 ≤5 分钟自动回流，免手动「从平台同步成员」
- [business/2026-10-16-超管成员保护-用户管理锁定.md](business/2026-10-16-超管成员保护-用户管理锁定.md) — 用户管理超管行锁定：is_super 标记下发 + 页面隐藏 + 本地哨兵 403（识别口径 2026-10-19 修订为 ID=1 硬锁定 + 标记双识别）
- [business/2026-10-17-侧边栏滚动条优化.md](business/2026-10-17-侧边栏滚动条优化.md) — 管理端深壳侧栏滚动条改 6px 圆角细拇指 + 透明轨道 + 稳定槽位；修 macOS overlay 不绘制与溢出切换菜单横向跳动
- [business/2026-10-19-用户列表准入刷新与超管ID1锁定.md](business/2026-10-19-用户列表准入刷新与超管ID1锁定.md) — 用户列表开/关准入后整表刷新（状态列以平台 admitted 为事实源）+ 超管 ID=1 本端硬锁定（守卫 + 列表归一化，不依赖平台标记下发）
- [business/2026-10-21-接口校验与数据边界加固.md](business/2026-10-21-接口校验与数据边界加固.md) — 无上限输入收口：trustedProxies 生效/http 读超时/BodyLimit/page.fullMax 夹取/AddMember 锁定角色提权修复/批量 ID max=200/物模型 256KB 上限等（决策记录 notes bug-fix 同名）

## 经验记忆（lessons）

- [lessons/2026-10-08-wire-bind-vs-interface-provider.md](lessons/2026-10-08-wire-bind-vs-interface-provider.md) — wire Provider 直接返回 biz 接口时不需要（也不能）再 wire.Bind
- [lessons/2026-10-08-frontend-route-namespace-collision.md](lessons/2026-10-08-frontend-route-namespace-collision.md) — 新增前端路由区先查既有菜单路径；守卫分流勿用裸前缀 startsWith
- [lessons/2026-10-08-httpserver-constructor-missing-field.md](lessons/2026-10-08-httpserver-constructor-missing-field.md) — HTTPServer 构造体漏字段赋值零告警，整域 handler nil panic→空体 500
- [lessons/2026-10-08-nselect-query-null-placeholder.md](lessons/2026-10-08-nselect-query-null-placeholder.md) — 搜索区 n-select 的 query 字段必须初始化 null；'' 会被当作已选值导致 placeholder 不显示（已晋升至 frontend-rules.md 组件规范）
- [lessons/2026-10-09-cross-repo-parallel-session-grep-misjudge.md](lessons/2026-10-09-cross-repo-parallel-session-grep-misjudge.md) — 跨仓并行会话中判「文件缺失/断头」前先绝对路径重 grep + mtime + 复跑 build；cwd 逐命令重置与 ugrep 静默 warning 会造成空结果误判
- [lessons/2026-10-08-obsolete-perm-codes-restore.md](lessons/2026-10-08-obsolete-perm-codes-restore.md) — 恢复旧菜单/权限 code 前必须先移出 obsoletePermCodes 淘汰清单，否则种子被 migrateLegacy 反复清掉
- [lessons/2026-10-09-vue-i18n-literal-braces.md](lessons/2026-10-09-vue-i18n-literal-braces.md) — locale 文案要展示字面 `{xxx}`（如 source_topic 占位符）时，调用处必须传同名字面参数，否则 vue-i18n 当插值变量解析
- [lessons/2026-10-09-menu-seed-softdelete-restore.md](lessons/2026-10-09-menu-seed-softdelete-restore.md) — 种子「Unscoped 判存在 + 缺失插入」必须配软删恢复分支；菜单软删会引发按钮孤儿化次生症状
- [lessons/2026-10-09-debug-server-port-occupancy.md](lessons/2026-10-09-debug-server-port-occupancy.md) — 调试残留/外部副本占用 dev 端口会让热重载静默失效（air bind 失败只进日志；vite 被别目录占用）；「改了不生效」先 lsof 确认监听者（已晋升至 relations/development-workflow.md 调试纪律）
- [lessons/2026-10-09-new-table-legacy-name-collision.md](lessons/2026-10-09-new-table-legacy-name-collision.md) — 新增本地表先查 migrateLegacy 清理名单，撞旧表名会被每次启动反复清空（tenant_user_roles→tenant_user_role_binds）
- [lessons/2026-10-09-setup-tdz-ref-before-declaration.md](lessons/2026-10-09-setup-tdz-ref-before-declaration.md) — script setup 顶层 TDZ：函数先调用、const ref 后声明，setup 抛错整页白屏；排查白屏先查顶层执行顺序
- [lessons/2026-10-10-nselect-remote-no-preload.md](lessons/2026-10-10-nselect-remote-no-preload.md) — 远程搜索 n-select 不预载首屏选项，「打开即空」会被用户感知为无数据；字段可见时以空关键词预载一次
- [lessons/2026-10-10-async-task-full-row-writeback.md](lessons/2026-10-10-async-task-full-row-writeback.md) — 异步任务收尾整行 Update 回写快照会覆盖执行期间并发的启停/编辑；只定向回写自己拥有的列
- [lessons/2026-10-10-precedent-tradeoff-reuse.md](lessons/2026-10-10-precedent-tradeoff-reuse.md) — 沿用先例的「残留无害」结论要按新场景消费方重查：有计数/聚合消费的绑定残留即有形伤害，需级联清理
- [lessons/2026-10-11-gui-smoke-admission-credential.md](lessons/2026-10-11-gui-smoke-admission-credential.md) — 管理端 GUI 冒烟凭据：admin 密码在平台 config（或播种日志）；「无权进入本系统」是准入 403 非密码错（已晋升至 development-workflow.md，冒烟改用 admitted=1 商户成员账号）
- [lessons/2026-10-12-format-drift-no-mechanical-gate.md](lessons/2026-10-12-format-drift-no-mechanical-gate.md) — 格式化规则无机械闸门必漂移：formatDateTime 两轮漏网后晋升 rg 自查模式进 frontend-utils（promoted）
- [lessons/2026-10-17-chromium-scrollbar-custom-style.md](lessons/2026-10-17-chromium-scrollbar-custom-style.md) — Chromium 自定义滚动条：`overflow-y: auto` 的 macOS overlay 不绘制；容器 `:hover` 改色不重绘，需拇指常驻 + 拇指自身 hover 增亮
- [lessons/2026-10-19-remote-fact-source-no-local-writeback.md](lessons/2026-10-19-remote-fact-source-no-local-writeback.md) — 列显示值为平台/远端事实源时，操作成功后改本地投影字段不代表显示值；须整表 load() 刷新

## 维护说明

- 新增记忆时创建文件并更新此索引
- 过时记忆及时清理，并同步清理索引条目
- 索引每条一行：文件相对路径 + 一句话摘要
