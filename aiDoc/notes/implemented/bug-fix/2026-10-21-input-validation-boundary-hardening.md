<!-- last-updated: 2026-10-21 -->
# 输入校验与数据边界加固（无上限输入收口）

> 路径：`aiDoc/notes/implemented/bug-fix/2026-10-21-input-validation-boundary-hardening.md`

## 问题

全仓输入校验审查发现三类系统性边界缺口：

1. **入口防护失效/缺失**：`server.trustedProxies` 配置从未调用 `SetTrustedProxies` 生效，gin 默认信任所有代理头 → 登录限流（10 次/分/IP）、LoginIPGuard 临时封禁、持久化 IP 黑名单、审计 IP 全部可被伪造的 `X-Forwarded-For` 绕过/投毒；`http.Server` 零读超时（Slowloris）；JSON body 无大小上限（XSSFilter 无上限 `io.ReadAll`，公开登录面单连接内存 DoS；multipart 超大 body 先写满临时目录才被拒）。
2. **提权与语义陷阱**：admission `AddMember` 未过滤内置锁定角色（`SetRoles` 路径有 `IsLockedRole` 过滤，Create 路径没有）→ 有「新增成员」权限的用户可直接绑定商户管理员角色（role 2）提权；job 启停接口 `oneof=0 1` 缺 `required`，空 body 零值 0 漏过校验 → 任务被静默停用。
3. **无上限输入放大**：`page_size=0` 全量约定无任何夹取（boundary.md 早已承诺「运行时有上限夹取」，代码未实现），约 10 个仓储对 `pageSize<=0` 整表拉取（日志/文件等大表）；批量 ID 列表（notice/admission/tenantuser/tenantmember/role/tenantdept/appuser/notify 各 IDs、Recipients）无数量上限，本地「先删后插」绑定替换可被超大数组写放大；物模型 Schema / 数据映射嵌套结构无大小约束直透平台；device data-events `limit` 无夹取；tenant-users dept_id 内存交集无翻页上限。

## 提案 / 决策

按「入口层硬性上限 + service binding 收紧 + biz 兜底」三层收口：

- **trustedProxies 生效**：`NewHTTPServer` 调 `e.SetTrustedProxies`，空列表 = 不信任任何代理头（直连部署安全默认），非法配置启动 fatal。
- **http.Server 读侧超时**：`readHeaderTimeoutSeconds/readTimeoutSeconds/idleTimeoutSeconds`（默认 10/30/120）；**不设 WriteTimeout**——Agent 调试对话为 SSE 流式长响应，全局写超时会误杀正常流。
- **全局 BodyLimit 中间件**（XSSFilter 之前）：非 multipart 按 `server.maxBodyMB`（默认 4MB）`http.MaxBytesReader` 硬限制；multipart 按 `storage.maxSizeMB + 1MB` 限制（精确大小校验仍在 handler/biz 的 fh.Size 检查，这里只挡临时目录写满）。Content-Length 预检超限直接 400（i18n `security.body_too_large`），未声明长度由 MaxBytesReader 读取时兜底。
- **AddMember 锁定角色过滤**：`filterAssignableRoles`（过滤 `IsLockedRole` + 去重）在 Create 前调用，与 SetRoles 口径一致。
- **`page_size=0` 全量夹取**：新增运行时系统参数 `page.fullMax`（默认 1000，系统参数页可调），`pageParams` 对显式 0 夹取到 fullMax——与 boundary.md 早已承诺的「运行时有上限夹取」对齐。
- **job StatusRequest**：改 `*int` + `required,oneof=0 1`（对齐 tenant SetStatus 写法），空 body → 400。
- **批量切片上限**：ID 类列表统一 `max=200`，notify Recipients `max=100,dive,max=320`（不加 email 格式——biz 仅校验含 `@`，避免破坏现有用法）；Mappings `max=500`。
- **devmodel 透传防护**：Mapping/TMProperty/TMParam 补 binding tag；物模型 Schema 与数据映射在透传前做序列化字节上限（256KB）校验，超限哨兵错误 `ErrPayloadTooLarge`（i18n `devmodel.payload_too_large`）。
- **其他**：device data-events limit 夹取 1–500 默认 50、遥测 IntervalSeconds/Limit 夹取（≤86400s / ≤1000）；dept_id 内存交集翻页上限 100 页（超限截断 + Warn 日志）；checkRolesInTenant/checkDeptsInTenant 校验前去重；blacklist Reason `max=255` 对齐 DB 列宽；permission UpdateRequest.Icon `max=512` 对齐 Create；notify Status 两处 oneof；admission repo LIKE 漏转义修复（全仓唯一一处未走 `EscapeLike`）；oplog/UA/导出失败原因截断保持字节预算但回退到 rune 边界（原直接 `s[:n]` 切断多字节 → utf8mb4 严格模式写日志失败静默丢失）；删死代码 `LoginRateMax`/`LoginRateWindow`（实际限流参数在 router.go 侧）。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 取消 `page_size=0` 全量语义，强制分页 | 前端引用数据（字典项、菜单、部门树等）依赖整表拉取，属破坏性契约变更；夹取到 fullMax 已消除无界风险 |
| 全局 WriteTimeout | Agent 调试对话是 SSE 流式端点，全局写超时误杀正常流；只设读侧超时 |
| 上传走全局统一 body 上限 | 上传上限（默认 20MB）天然大于 JSON 上限（4MB），必须分档；精确校验保持 handler/biz 双层 |
| Recipients 加 `dive,email` 强格式 | biz 历史口径仅校验含 `@`，强格式会破坏存量用法；只收数量与单元素长度 |
| SSRF 出向 allowlist（provider base_url / webhook） | RBAC 保护的管理端功能，风险可控；记为已知风险待平台侧统一考虑，本次不做 |

## 验收标准

- [x] 伪造 `X-Forwarded-For` 不再污染 `c.ClientIP()`——已冒烟：带 `X-Forwarded-For: 1.2.3.4` 的失败登录，login_logs 记录 IP 为直连地址 127.0.0.1
- [x] 超 `maxBodyMB` 的 JSON body → 400 `security.body_too_large`——已冒烟：公开登录面 5MB body 返回 400；multipart 路径代码审查确认（上限 = storage.maxSizeMB + 1MB）
- [x] 空 body / 越界值调 `PUT /jobs/:id/status` → 400，正常 status=1 → 200——已冒烟
- [x] 批量 ID 列表超 200 → 400——已冒烟（`/users/:id/roles` 249 个 ID）
- [x] AddMember 传锁定角色 ID 被过滤——代码审查（filterAssignableRoles 与 SetRoles 同口径），无端到端用例
- [ ] `page_size=0` 返回行数 ≤ `page.fullMax`——夹取逻辑与已验证的 sizeMax 同一代码路径，代码审查确认，未做超千行数据端到端验证
- [x] `go build ./...` 通过；middleware/admission/logger 聚焦测试 passed（其余包无测试文件）

## 风险与后果

- 行为收紧：超限请求从「静默接受/截断」变为 400，前端正常用法均在上限内（已核实 job 启停恒传 status）；若有脚本依赖超大批量绑定（>200 ID/次）需分批
- `page_size=0` 语义从「真全量」变为「全量上限 1000 行」，超大租户/日志表导出应走异步导出通道（已有独立限流）
- 反向代理部署必须在 `server.trustedProxies` 配置代理地址，否则审计/限流取到的是代理 IP（配置注释已写明）
- 已知遗留：SSRF 出向面、admission repo 逐行 loadRoles N+1（perf，非边界问题）

## 交叉链接

- 契约更新：[../../contracts/boundary.md](../../contracts/boundary.md)（分页夹取、批量上限、body 上限）
- 锁定角色机制：[../feature/2026-10-16-superadmin-member-lock.md](../feature/2026-10-16-superadmin-member-lock.md)、[../feature/2026-10-19-superadmin-id1-hard-lock.md](../feature/2026-10-19-superadmin-id1-hard-lock.md)
- 登录代理与 IP 防线：[../architecture/2026-10-08-admin-login-backend-proxy.md](../architecture/2026-10-08-admin-login-backend-proxy.md)
