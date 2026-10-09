export interface R<T = any> {
  code: number
  msg: string
  data: T
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_at: string
}

// 图形验证码（enabled=false 表示服务端已停用，前端隐藏验证码表单）
export interface CaptchaInfo {
  captcha_id: string
  captcha_image: string // PNG base64，无 data: 前缀，展示时需自行拼接 dataURL
  enabled: boolean
}

// 个人信息（平台身份 + 本系统准入状态；账号体系在平台上，本地只做准入投影）
export interface UserInfo {
  id: number // 平台用户 ID
  username: string
  nickname: string
  email: string
  admitted: boolean // 本系统准入状态
  role_names?: string[]
}

export interface Permission {
  id: number
  name: string
  localized_name?: string // 按当前语言的展示名（语言包命中时才有；展示用，编辑表单仍用 name）
  code: string
  type: 'dir' | 'menu' | 'button' // dir 目录分组 | menu 菜单页面 | button 按钮权限点（method/path 非空时同时参与后端 RBAC 校验）
  method: string
  path: string
  parent_id: number
  icon: string
  sort: number
}

export interface MenuNode {
  id: number
  name: string
  code: string
  type: 'dir' | 'menu' // dir 目录分组（无路由）| menu 菜单页面
  path: string
  icon: string
  sort: number
  children: MenuNode[] | null
}

// 菜单搜索命中项（顶栏命令面板；parents 为父级链提示，不含自身）
// dir 标记目录（含子菜单、无路由，选中时软提示选择具体菜单）
export interface MenuHit {
  name: string
  path: string
  icon: string
  parents: string
  dir?: boolean
  depth?: number
}

export interface Role {
  id: number
  name: string
  remark: string
  permission_ids?: number[] | null
}

export interface PageResult<T> {
  list: T[]
  page: { page: number; page_size: number; total: number }
}

// 用户准入投影（平台用户在本系统的准入记录：开关 + 本地角色）
export interface AdmissionProjection {
  id: number
  platform_user_id: number
  username: string
  nickname: string
  email: string
  enabled: boolean // 准入开关
  role_ids: number[] | null
  created_at: string
  updated_at: string
}

// 成员列表合并行（来源=平台本商户绑定成员 + 本地准入投影可空）
export interface MemberRow {
  platform_user_id: number
  username: string
  nickname: string
  platform_status: number // 平台侧账号状态：1 启用 0 禁用（禁用=平台全局）
  is_admin: boolean // 平台侧商户管理员标记（实时事实源；本地角色2在其登录/同步后对齐）
  admitted: boolean // 业务端准入状态（平台侧实时事实源；本端开关写回）
  bound_at: string // 平台侧绑定时间
  projection: AdmissionProjection | null // null=平台已绑定但本系统未准入
}

// 登录日志（一次登录尝试 = 一条）
export interface LoginLogInfo {
  id: number
  username: string // 尝试登录的用户名（可能不存在）
  ip: string
  user_agent: string
  device: string // web | app
  status: number // 1 成功 0 失败
  msg: string // 失败原因（成功为空）
  created_at: string
}

// 操作日志（一条写请求审计）
export interface OperationLogInfo {
  id: number
  user_id: number
  username: string
  method: string // POST / PUT / DELETE / PATCH
  path: string // 实际请求路径
  route: string // 路由模板
  action: string // 中文动作名
  params: string // 参数摘要（敏感字段已脱敏）
  ip: string
  user_agent: string
  status_code: number
  latency_ms: number
  created_at: string
}

// 日志列表响应：列表 + 分页 + 保留天数（页面展示保留说明用）
export interface LogPageResult<T> {
  list: T[]
  page: { page: number; page_size: number; total: number }
  retention_days: number
}

// 异步导出记录（一行 = 一次导出任务；仅本人可见）
export interface ExportRecord {
  id: number
  biz: 'user' | 'op_log'
  name: string
  status: 'pending' | 'running' | 'done' | 'failed'
  size: number
  rows: number
  truncated: boolean // 超过单文件行数上限被截断
  error: string // 失败原因（成功为空）
  created_at: string
  finished_at: string
}

// IP 黑名单项
export interface BlacklistItem {
  id: number
  ip: string
  reason: string
  source: string // manual | auto
  expire_at: string | null // 为空字符串或 null 表示永久
  creator_name: string
  created_at: string
}

// 租户（platform_id 非零 = 已同步至 embodied-platform；status: 1 启用 0 禁用；code 创建后不可修改）
export interface Tenant {
  id: number
  platform_id: number
  name: string
  code: string
  contact_name: string
  contact_phone: string
  remark: string
  status: number
  created_at: string
  updated_at: string
}

// 应用用户（含租户关联，不含密码；status: 1 启用 0 禁用）
export interface AppUser {
  id: number
  username: string
  nickname: string
  phone: string
  email: string
  status: number
  tenant_ids: number[]
  tenant_names: string[]
  created_at: string
  updated_at: string
}

// 租户用户（平台第四套身份：租户门户运营账号，单租户绑定；本地无数据面，
// 经开放面实时消费；id/tenant_id/role_ids 均为平台口径）
export interface TenantUserAccount {
  id: number
  tenant_id: number
  tenant_name: string
  username: string
  nickname: string
  phone: string
  email: string
  status: number
  role_ids: number[]
  created_at: string
  updated_at: string
}

// 租户角色（租户作用域 RBAC；perm_codes 为平台租户门户权限点目录内的码）
export interface TenantRole {
  id: number
  tenant_id: number
  tenant_name: string
  name: string
  code: string
  remark: string
  perm_codes: string[]
  created_at: string
  updated_at: string
}

// 租户门户权限点目录项（group 为资源域：device/alarm/dataset/appuser/member）
export interface TenantUserPermDef {
  code: string
  group: string
}

// 文件元数据（object_key 不下发；driver: platform | local 历史存量）
export interface FileInfo {
  id: number
  driver: string
  name: string
  ext: string
  size: number
  content_type: string
  uploader_id: number
  uploader_name: string
  created_at: string
}

// ---- 服务器状态监控 ----
export interface ServerHost {
  hostname: string
  os: string
  platform: string
  platform_version: string
  kernel_arch: string
  kernel_version: string
  boot_time: number // unix 秒
}

export interface ServerCPU {
  model_name: string
  cores: number
  percent: number // 总体使用率 0-100（后端 3s 窗口差值）
  per_core: number[] // 每核使用率 0-100
}

export interface ServerMem {
  total: number
  used: number
  available: number
  used_percent: number
  swap_total: number
  swap_used: number
  swap_percent: number
}

export interface ServerDisk {
  device: string
  mount: string
  fstype: string
  total: number
  used: number
  free: number
  used_percent: number
}

export interface ServerNet {
  name: string
  bytes_sent: number // 累计发送字节
  bytes_recv: number // 累计接收字节
  send_rate: number // B/s
  recv_rate: number // B/s
}

export interface ServerGoRuntime {
  version: string
  goroutines: number
  heap_alloc: number
  sys_memory: number
  gc_count: number
  gc_pause_ms: number
  process_start: number // unix 秒
}

export interface ServerStatus {
  time: number // 快照 unix 秒
  uptime: number // 主机已运行秒数
  host: ServerHost | null
  cpu: ServerCPU
  memory: ServerMem
  disks: ServerDisk[]
  net: ServerNet[]
  go: ServerGoRuntime
}

// ---- 智能体（LLM 配置底座）----

// LLM 供应商配置（api_key 全链路仅展示掩码，明文只在创建/编辑时提交）
export interface AgentProvider {
  id: number
  name: string
  code: string
  base_url: string
  api_key_mask: string // 展示掩码（如 sk-****abcd；空=未配置，本地服务可不需要）
  protocol: string
  remark: string
  status: number
  created_at: string
  updated_at: string
}

// 供应商下的模型配置
export interface AgentModel {
  id: number
  provider_id: number
  name: string // 上游模型标识（如 glm-4.7）
  display_name: string
  context_window: number // 上下文窗口（token，0=未知）
  max_output: number // 单次最大输出（token，0=上游默认）
  supports_tools: boolean
  input_price: number // 每千 token 输入单价（0=未设置）
  output_price: number
  remark: string
  status: number
  created_at: string
  updated_at: string
}

// Agent 配置（业务按 code 稳定引用）
export interface AgentInfo {
  id: number
  name: string
  code: string
  model_id: number
  system_prompt: string
  temperature: number // 0~2；0=上游默认
  top_p: number // 0~1；0=上游默认
  max_tokens: number // 0=上游默认
  tools: string[] // 绑定的本地工具名（function calling）
  skills: string[] // 绑定的技能 code（聊天注入 system prompt）
  remark: string
  status: number
  created_at: string
  updated_at: string
}

// 连通性测试结果
export interface AgentTestResult {
  content: string
  model: string
  latency_ms: number
  usage: { prompt_tokens: number; completion_tokens: number; total_tokens: number }
}

// 调试对话消息（system 由 Agent 配置注入，前端仅传 user/assistant）
export interface AgentChatMessage {
  role: 'user' | 'assistant'
  content: string
}

// ---- 设备（平台开放面契约镜像，字段与平台 SDK types.go 一致） ----

export interface Device {
  id: number
  sn: string
  name: string
  model_id: number
  tenant_id: number
  status: string // inactive/active/disabled/retired
  online: boolean
  transport: string // ''跟随平台默认 | socket | mqtt
  firmware_version?: string
  hardware_version?: string
  service_versions?: Record<string, string>
  last_seen_at: string
  created_at: string
  updated_at: string
}

export interface DeviceShadow {
  device_id: number
  desired: string // JSON 文本
  reported: string // JSON 文本
  updated_at: string
}

export interface DeviceCommand {
  id: number
  device_id: number
  command_type: string
  params: string
  priority: number // 0 急停（最高危）/1 运维 /2 Agent /3 业务
  status: string // pending/dispatched/acked/succeeded/failed/cancelling/cancelled/preempted/timeout
  caller: string
  idempotency_key?: string
  result?: string
  trace_id?: string
  created_at: string
  updated_at: string
}

export interface DataEvent {
  id: number
  device_id: number
  event_type: string
  payload: string
  source_topic: string
  occurred_at: string
}

export interface TelemetryHistory {
  items: { metric: string; value: number; str_value?: string; ts: string }[]
  next_marker?: string
}

// ---- 设备型号（平台管理面契约镜像） ----

export interface DeviceModel {
  id: number
  code: string
  name: string
  tm_node_id: number
  tm_version_id: number
  tm_node_name?: string
  tm_version_no?: number
  status: number // 1 启用 2 禁用
  merchant_id: number // 0=通用 >0=商户独立归属（开放面读=通用+本商户，写仅本商户）
  manufacturer: string
  description: string
  transport: string
  created_at: string
  updated_at: string
}

// 物模型节点（layer: base/category/model/instance；读=通用+本商户，写仅本商户独立节点）
export interface TMNode {
  id: number
  layer: string
  parent_id: number
  code: string
  name: string
  status: number // 1 启用 2 禁用
  merchant_id: number // 0=通用 >0=商户独立归属
}

// 物模型属性要素
export interface TMProperty {
  data_type: string // int / float / bool / string / enum / json
  unit?: string
  writable?: boolean
  enum?: string[]
  min?: number
  max?: number
  description?: string
}

// 服务入参/事件出参定义
export interface TMParam {
  data_type: string
  required?: boolean
  enum?: string[]
  description?: string
}

// 服务要素
export interface TMService {
  call_type: string // sync / async
  params?: Record<string, TMParam>
  description?: string
}

// 事件要素
export interface TMEvent {
  params?: Record<string, TMParam>
  description?: string
}

// 物模型 Schema（三类要素以名称为主键）
export interface TMSchema {
  properties?: Record<string, TMProperty>
  services?: Record<string, TMService>
  events?: Record<string, TMEvent>
}

// 物模型版本（GET /thing-models/:id/versions 仅返回 published 选择器视图（不含 schema）；
// 写面动作（建草稿/发布/回滚）返回含 draft 状态与 schema 的完整版本）
export interface TMVersion {
  id: number
  node_id: number
  version: number
  schema?: TMSchema | null
  status: 'draft' | 'published'
  parent_versions?: string // 父链快照 JSON 文本（[{node_id,version_id}]）
  revert_of?: number | null
  published_at: string
  created_at: string
  updated_at?: string
}

// 合并解析结果（schema + 参与合并的链路版本）
export interface TMResolveResult {
  schema: TMSchema | null
  chain: { node: TMNode; version: TMVersion }[]
}

// 继承状态（基线的父链快照是否落后于各父层最新已发布版本）
export interface TMInheritanceStatus {
  up_to_date: boolean
  baseline: 'draft' | 'published' | 'none' | string
  updates?: {
    node: TMNode
    snapshot_version: TMVersion | null
    latest_version: TMVersion
  }[]
}

// 创建物模型节点入参（merchant_id 服务端注入，入参不收；父链校验平台兜底）
export interface TMNodeCreateInput {
  layer: 'base' | 'category' | 'model' | 'instance'
  parent_id?: number
  code: string
  name: string
}

// 更新物模型节点入参（code 与 merchant_id 创建后不可改）
export interface TMNodeUpdateInput {
  name: string
  status: number // 1 启用 2 禁用
}

// ---- 数据映射（平台开放面契约镜像；读=通用+本商户，写仅本商户独立资源） ----

// 单条映射规则：source_topic → 数据分类 + 字段提取/触发
//（source_topic 支持设备占位符 {sn}/{device_id}/{name}/{model_id}）
export interface Mapping {
  source_topic: string // 通道内主题（socket 帧 topic / MQTT topic）
  topic_match?: 'exact' | 'glob' | '' // exact（默认，空=省略）| glob
  data_category: 'shadow' | 'telemetry' | 'event' | 'alarm' | 'media' | string
  data_type: string // shadow=物模型属性名；event=物模型事件名；telemetry/alarm=自定义
  sample_rate_hz?: number // 0/缺省=不限流
  field_extract?: Record<string, string> // 目标字段 → 提取表达式
  fields?: string[]
  trigger?: string
}

// 数据映射资源（merchant_id=0 通用 / >0 商户独立）
export interface DataMappingDef {
  id: number
  name: string
  merchant_id: number
  created_at: string
  updated_at: string
}

// 映射版本（资源内版本号自增；draft→published 发布不可变；revert_of 记录回滚来源版本 ID）
export interface DataMappingVersion {
  id: number
  def_id: number
  version: number
  label?: string
  mappings: Mapping[]
  status: 'draft' | 'published'
  revert_of?: number | null
  published_at: string
  created_at: string
  updated_at: string
}

// 创建映射资源应答：资源 + 首版草稿
export interface DataMappingCreateResult {
  def: DataMappingDef
  version: DataMappingVersion
}

// 创建映射资源入参（资源 + 首版草稿一步完成；merchant_id 服务端注入，入参不收）
export interface DataMappingCreateInput {
  name: string
  label?: string
  mappings: Mapping[]
}

// 新建草稿入参（单草稿制：已有草稿平台 409）
export interface DataMappingDraftInput {
  label?: string
  mappings: Mapping[]
}

// 更新草稿入参（label 省略=不修改；仅 draft 可改）
export interface DataMappingDraftUpdateInput {
  label?: string
  mappings: Mapping[]
}

// 对话会话（本人数据；AgentName 冗余，Agent 删除后历史仍可读）
export interface AgentConversation {
  id: number
  user_id: number
  agent_id: number
  agent_name: string
  title: string
  last_msg_at: string
  created_at: string
  updated_at: string
}

// 会话消息（追加流水）
export interface AgentConversationMessage {
  id: number
  conversation_id: number
  role: 'user' | 'assistant'
  content: string
  total_tokens: number // assistant 消息的 usage.total_tokens
  created_at: string
}

// 用量统计（按日聚合 + Top Agent）
export interface UsageDailyPoint {
  date: string
  calls: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cost: number
}

export interface UsageAgentPoint {
  agent_id: number
  agent_name: string
  calls: number
  total_tokens: number
  cost: number
}

export interface UsageStats {
  days: UsageDailyPoint[]
  agents: UsageAgentPoint[]
  calls: number
  tokens: number
  cost: number
}

// 数据字典
export interface DictType {
  id: number
  name: string
  code: string
  remark: string
  status: number
  created_at: string
  updated_at: string
}

export interface DictItem {
  id: number
  type_id: number
  label: string
  value: string
  sort: number
  remark: string
  status: number
  created_at: string
  updated_at: string
}

// 系统参数（运行时可调）
export interface SysConfig {
  key: string
  value: string
  type: 'string' | 'number' | 'bool'
  description: string
  updated_at: string
}

// 通知公告
export interface NoticeInfo {
  id: number
  title: string
  content: string // markdown
  level: 'info' | 'warning' | 'important'
  scope: 'all' | 'roles' | 'users' // 送达范围：全体/按角色/按用户
  role_ids?: number[] // scope=roles 时回显（与 role_names 同序）
  role_names?: string[]
  user_ids?: number[] // scope=users 时回显（与 user_names 同序）
  user_names?: string[]
  publish_at: string
  expire_at?: string | null
  creator_name: string
  created_at: string
  updated_at: string
  has_read?: boolean
}

// 公告送达范围选项（发布表单用）
export interface NoticeRoleOption {
  id: number
  name: string
}
export interface NoticeUserOption {
  id: number
  username: string
  nickname: string
}

// 告警通知 —— 渠道
export type NotifyChannelType = 'email' | 'webhook' | 'wecom' | 'dingtalk' | 'feishu'
export interface NotifyChannel {
  id: number
  name: string
  type: NotifyChannelType
  status: number
  smtp_host?: string
  smtp_port?: number
  smtp_user?: string
  smtp_password_mask?: string
  smtp_from?: string
  recipients?: string[]
  webhook_url?: string
  webhook_secret_mask?: string
  created_at: string
  updated_at: string
}

// 告警通知 —— 规则
export interface NotifyRule {
  id: number
  name: string
  source: 'job_failed' | 'monitor' | 'ip_autoban'
  metric: 'cpu' | 'mem' | 'swap' | ''
  threshold: number
  channel_ids: number[]
  cooldown_seconds: number
  status: number
  remark: string
  last_triggered_at?: string | null
  created_at: string
  updated_at: string
}

// 告警通知 —— 发送记录
export interface NotifyRecord {
  id: number
  channel_id: number
  channel_name: string
  channel_type: 'email' | 'webhook'
  rule_name: string
  source: 'job_failed' | 'monitor' | 'ip_autoban' | 'test'
  title: string
  content: string
  status: 'sent' | 'failed'
  error?: string
  duration_ms: number
  created_at: string
}

// 告警通知 —— 渠道测试结果
export interface NotifyTestResult {
  status: 'sent' | 'failed'
  error?: string
  duration_ms: number
}

// 定时任务
export interface JobInfo {
  id: number
  name: string
  cron: string
  handler_key: string
  params?: string
  remark: string
  status: number
  last_run_at?: string | null
  created_at: string
  updated_at: string
}

export interface JobHandler {
  key: string
  description: string
}

export interface JobLog {
  id: number
  job_id: number
  job_name: string
  handler_key: string
  status: 'success' | 'failed'
  output: string
  duration_ms: number
  started_at: string
}

// 仪表盘聚合数据
export interface DashboardStats {
  cards: {
    users: number
    roles: number
    online: number
    today_logins: number
    today_login_count: number // 今日登录成功次数
  }
  login_trend: { date: string; total: number; success: number }[]
  op_trend: { date: string; total: number }[]
  recent_logins: { username: string; ip: string; status: string; created_at: string }[]
}

// 监控历史快照
export interface MonitorHistoryPoint {
  ts: number
  cpu_percent: number
  mem_percent: number
  swap_percent: number
  net_send_rate: number
  net_recv_rate: number
}

// ---- MCP 服务 ----

// 自定义请求头（明文非敏感；敏感凭证走 token 加密通道）
export interface McpHeader {
  key: string
  value: string
}

export interface McpServer {
  id: number
  name: string
  code: string // 稳定引用（Agent 以 mcp:<code>:<tool> 绑定工具）
  transport: 'streamable_http' | 'sse'
  base_url: string
  headers: McpHeader[]
  token_mask: string // 展示掩码；空=未配置凭证
  remark: string
  status: number
  created_at: string
  updated_at: string
}

// 写入参数（token 明文仅写入链路；编辑时留空=保持不变）
export type McpServerInput = Partial<Omit<McpServer, 'headers' | 'token_mask'>> & {
  headers?: McpHeader[]
  token?: string
}

export interface McpTestResult {
  ok: boolean
  server_name: string
  server_version: string
  protocol_version: string
  tools: { name: string; description: string }[]
  latency_ms: number
  error?: string
}

// 可绑定工具分组（本地内置 + 各启用 MCP 服务）
export interface AgentToolGroups {
  builtin: string[]
  mcp: { server_code: string; server_name: string; tools: { name: string; description: string }[] }[]
}

// ---- 技能（多文件技能包） ----

// 附属文件（回显含内容；列表场景内容仅作元信息展示）
export interface SkillFile {
  path: string
  content: string
}

export interface SkillInfo {
  id: number
  name: string
  code: string // 稳定引用（Agent 绑定用）
  description: string
  instruction: string // 主指令（SKILL.md 等价物，Markdown）
  files: SkillFile[]
  remark: string
  status: number
  created_at: string
  updated_at: string
}

// 写入参数（files 随技能整体提交替换）
export type SkillInput = Partial<Omit<SkillInfo, 'files'>> & { files?: SkillFile[] }
