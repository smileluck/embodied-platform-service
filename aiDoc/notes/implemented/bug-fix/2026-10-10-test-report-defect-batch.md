<!-- last-updated: 2026-10-10 -->
# 测试报告缺陷批次修复（DEF-01~15，SmileX Admin 测试用例报告 2026-10-10）

## 问题

外部测试报告（287 用例、15 项缺陷/风险）针对本服务管理端（:28170/:28180）暴露一批接口层缺陷，
集中在：入参校验缺位（分页/hours/租户 code/查询长度/XSS）、错误语义混乱（refresh 401 透传平台
内部兜底文案、资源不存在状态码不一、SDK 错误串裸透传）、运维信息泄露（/monitor 主机名）、
导出无频控、安全响应头缺 HSTS、服务端不 trim 依赖前端 deepTrim。

## 提案 / 决策

按「入口层 WAF 式统一防线 + 各 handler 语义修正 + 商户授权补齐」三层落地（全部已实测验证）：

1. **查询入参统一防线**（`middleware/security.go` SQLInjectionGuard 扩展）：值先 TrimSpace
   （与前端 deepTrim 同口径）；单值超 256 rune → 400 `security.param_too_long`；
   `security.ContainsXSSPayload`（危险标签/标签内事件属性/javascript: 伪协议，`pkg/security`）
   命中 → 400 `security.invalid_chars`；`page`/`page_size` 非整数、page<1、page_size<0 → 400
   （page_size=0 为「全量」约定值放行，本地仓储原生支持）。
2. **body 统一 trim**：XSSFilter 清洗 JSON 字符串值时叠加 TrimSpace（password 类 key 跳过，
   避免静默改写密码）。
3. **分页全量语义贯通代理链路**（`platformsdk/client.go normPageSize`）：开放面列表把
   page_size=0 翻译为 200 大页（平台侧多数列表按字面 0 回空集）；thing-models 平台原生
   支持 0=全量，不经该归一。
4. **租户 code 字符集校验**（biz/tenant）：`^[a-zA-Z0-9_-]{2,64}$`，ErrInvalidTenantCode →
   400 `tenant.code_invalid`。
5. **refresh 401 语义修正**（管理端 + 租户门户）：平台 401 统一回 401 `auth.refresh_failed`
   （"登录已过期，请重新登录"），不透传平台内部兜底文案（根因在平台 refresh 分支 msg，
   本侧兜底；平台侧修复另行跟进）。
6. **资源不存在 404 统一**：roles getRole 走 roleErr 并对 ErrRoleNotFound 回 404；
   设备代理链平台 404 → 404 `device.not_found`；准入链 ErrPlatformNotBound → 404 `user.not_bound`。
7. **SDK 错误脱敏**（admissionErr）：`*platformsdk.Error` 先经 normalizeSDKError 归一为平台
   信封错误再透传（HTTP 状态 + msg），杜绝 "openapi: http 403 code 403: ..." 裸串到前端。
8. **monitor hours 严格校验**：非正整数 400；>72h 由 biz 夹取到 72h（原为回 24h）。
9. **/monitor 主机名脱敏**：HostInfo.Hostname 不再回真实主机名（前端显示 '—'）。
10. **导出频控**：提交路由挂 per-user 10 次/分钟限流（rl:export:）+ biz 层单用户
    pending/running ≥5 拒绝（ErrTooManyActive → 429）。
11. **HSTS**：SecurityHeaders 在 TLS（或 X-Forwarded-Proto=https）时下发 max-age=31536000。
12. **商户授权补齐（DEF-09/11 根因）**：商户 wujie-ego 缺 `mapping:*`（6 项）与
    `user:setAdmission` scopes，致数据映射菜单可见但开放面 403、准入开关透传 403——
    已在平台库补授权（开放面验签实时查库，即时生效）。生产部署须同样补齐
    （漏配 scopes 是部署联动要求，见项目集 AGENTS.md）。

DEF-05（用户名枚举）复核为误报：平台对「存在用户+错密码」与「不存在用户」均回 401
"用户名或密码错误"；报告中的 429 是连续探测触发本侧 LoginIPGuard/限流的时序产物，
两种场景限流口径一致，无枚举面，未改动。

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| pageParams 改签名返回 error（35 处调用点全改） | 入参校验放全局中间件一处生效，diff 与回归面小得多 |
| 平台代理列表 page_size=0 做分批循环真全量 | 复杂度高；前端从不对代理列表传 0，200 大页（平台仍按 page.sizeMax 夹取）已消除空集不一致 |
| 修平台 refresh 分支的 msg（跨仓库） | 属平台仓库变更，须按其 harness 流程另行留痕落地；本侧 401 兜底文案已消除用户可见缺陷 |
| 按租户类型隐藏数据映射菜单（DEF-09 备选） | 菜单可见性与商户能力本应对齐——商户被授予 mapping:* 后菜单与接口即一致，授权补齐是正解 |

## 验收标准

- [x] page=abc/0、page_size=-1/abc → 400；page_size=0 在 /users 返回全量（实测 3/3）
- [x] kw=`<script>alert(1)</script>` → 400；kw 5000 字符 → 400
- [x] 租户 code「含中文 空格!」/「bad code!!」→ 400；合法 code 创建正常（临时租户已清理）
- [x] POST /auth/refresh 坏 token → 401「登录已过期，请重新登录」（管理端 + 门户）
- [x] /devices/999999999 → 404「设备不存在」；/roles/999999999 → 404「角色不存在」
- [x] /users/999999999/admission → 404「该用户未绑定本商户」（无 SDK 错误串裸透传）
- [x] /monitor host.hostname 为空；hours=abc → 400；hours=99999 夹取 72h 窗口
- [x] 连续 12 次导出：前 10 次成功，第 11/12 次 429（探测记录已全部清理复原）
- [x] X-Forwarded-Proto=https 时响应含 Strict-Transport-Security；明文 HTTP 不含
- [x] 角色名「  x  」创建后落库为「x」（服务端 trim）
- [x] go build / go vet / go test ./... 全绿；新增 pkg/security 单测（XSS/SQL 特征正负例）
- [x] 前端回归：/permissions?page_size=0 仍全量（226 条）、vite 代理冒烟 200

## 风险与后果

- 查询入参 WAF 拦截存在误报可能（如刻意搜索含 `<script` 文本的日志）；正则已收窄到明确
  载体，误报面小，属可接受权衡。
- 代理列表 page_size=0 语义为「≤平台单页上限的首屏全量」而非无限全量；若未来商户成员/设备
  超过平台单页上限且前端需要真全量下拉，再升级为分批循环。
- 服务端 trim 对 password 类字段跳过：密码含首尾空格的用户输入会原样进入平台校验
  （与前端 deepTrim 行为存在理论分叉，deepTrim 同样跳过密码则为一致）。
- 生产环境必须为商户补 mapping:*/user:setAdmission scopes，否则数据映射页/准入开关 403 复现。

## 交叉链接

- 项目集协调：`AGENTS.md`（emb 项目集根）「部署联动」行——漏配 scopes 则管理页 403
- 相关代码：`internal/server/middleware/security.go`、`internal/platformsdk/client.go:normPageSize`、
  `internal/biz/tenant/usecase.go:tenantCodePattern`、`internal/server/handler_auth.go:refreshProxyErr`、
  `internal/biz/export/usecase.go:maxActivePerUser`
- 平台侧待跟进：embodied-platform refresh 无效 token 分支 msg 使用内部兜底文案（开放面 404 同）
