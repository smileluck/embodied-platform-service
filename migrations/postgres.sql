-- PostgreSQL 建表参考（种子数据由程序启动时写入）
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  username VARCHAR(64) UNIQUE NOT NULL,
  password VARCHAR(128) NOT NULL,
  nickname VARCHAR(64) DEFAULT '',
  phone VARCHAR(32) DEFAULT '',
  email VARCHAR(128) DEFAULT '',
  status INT DEFAULT 1,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS roles (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(64) UNIQUE NOT NULL,
  remark VARCHAR(255) DEFAULT '',
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS permissions (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  type VARCHAR(16) DEFAULT 'menu', -- menu 菜单 | button 按钮权限（api 已废弃，启动迁移自动转为 button）
  method VARCHAR(16) DEFAULT '',
  path VARCHAR(255) DEFAULT '',
  parent_id BIGINT DEFAULT 0,
  icon VARCHAR(512) DEFAULT '',
  sort INT DEFAULT 0,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_roles (
  user_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id BIGINT NOT NULL,
  permission_id BIGINT NOT NULL,
  PRIMARY KEY (role_id, permission_id)
);

-- 文件元数据表（对象本体在 driver 对应的存储后端）
CREATE TABLE IF NOT EXISTS files (
  id BIGSERIAL PRIMARY KEY,
  driver VARCHAR(16) NOT NULL,          -- local | oss | cos | tos | minio（落库时的存储后端）
  object_key VARCHAR(512) NOT NULL UNIQUE, -- 服务端生成的对象 key
  name VARCHAR(255) NOT NULL,           -- 原始文件名
  ext VARCHAR(16) DEFAULT '',
  size BIGINT DEFAULT 0,
  content_type VARCHAR(128) DEFAULT '',
  uploader_id BIGINT DEFAULT 0,
  uploader_name VARCHAR(64) DEFAULT '',
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_files_driver ON files (driver);
CREATE INDEX IF NOT EXISTS idx_files_ext ON files (ext);
CREATE INDEX IF NOT EXISTS idx_files_uploader_id ON files (uploader_id);
CREATE INDEX IF NOT EXISTS idx_files_deleted_at ON files (deleted_at);

-- 异步导出任务记录表（产物本体在 driver 对应的存储后端；无软删，保留期清理为物理删除）
CREATE TABLE IF NOT EXISTS export_records (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT DEFAULT 0,             -- 任务归属用户
  biz VARCHAR(32) DEFAULT '',           -- 业务类型（user / login_log / op_log）
  name VARCHAR(255) DEFAULT '',         -- 展示名（兼作下载文件名，提交时已按语言翻译）
  locale VARCHAR(16) DEFAULT '',        -- 提交时语言快照（worker 表头/行内值翻译用）
  params TEXT,                          -- 查询条件快照（JSON）
  driver VARCHAR(16) DEFAULT '',        -- 产物落库时的存储后端
  object_key VARCHAR(512) DEFAULT '',   -- 产物对象 key
  size BIGINT DEFAULT 0,                -- 产物字节数（含 BOM）
  rows INT DEFAULT 0,                   -- 已导出数据行数（不含表头）
  status VARCHAR(16) DEFAULT 'pending', -- pending | running | done | failed
  truncated BOOLEAN DEFAULT FALSE,      -- 触及大小/行数上限被截断
  error VARCHAR(512) DEFAULT '',        -- 失败原因（成功为空）
  created_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ               -- 完成/失败时间（未结束为 NULL）
);
CREATE INDEX IF NOT EXISTS idx_export_records_user_id ON export_records (user_id);
CREATE INDEX IF NOT EXISTS idx_export_records_status ON export_records (status);
CREATE INDEX IF NOT EXISTS idx_export_records_created_at ON export_records (created_at);

-- IP 黑名单表（管理员手工维护的持久化封禁；软删即解封留痕）
CREATE TABLE IF NOT EXISTS ip_blacklist (
  id BIGSERIAL PRIMARY KEY,
  ip VARCHAR(64) NOT NULL UNIQUE,       -- 单个 IP（不支持 CIDR）
  reason VARCHAR(255) DEFAULT '',       -- 封禁原因
  source VARCHAR(16) NOT NULL DEFAULT 'manual', -- manual | auto（登录连续失败自动封禁）
  expire_at TIMESTAMPTZ,                -- 过期时间（NULL 为永久封禁）
  creator_id BIGINT DEFAULT 0,
  creator_name VARCHAR(64) DEFAULT '',
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_ip_blacklist_deleted_at ON ip_blacklist (deleted_at);

-- 商户表（开放 API 授权；app_secret 只存哈希 SHA-256(AppKey + ":" + secret)，软删留痕）
CREATE TABLE IF NOT EXISTS merchants (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  code VARCHAR(64) UNIQUE NOT NULL,     -- 商户编码（创建后不可改）
  app_key VARCHAR(64) UNIQUE NOT NULL,  -- 开放 API 调用凭证 key（mk_ 前缀）
  app_secret_hash VARCHAR(128) NOT NULL, -- secret 哈希，明文仅创建/重置时返回一次
  contact_name VARCHAR(64) DEFAULT '',
  contact_phone VARCHAR(32) DEFAULT '',
  contact_email VARCHAR(128) DEFAULT '',
  status INT DEFAULT 1,                 -- 1 启用 2 禁用
  remark VARCHAR(255) DEFAULT '',
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_merchants_deleted_at ON merchants (deleted_at);

-- 开放 API 调用日志表（无软删，保留期清理为物理删除）
CREATE TABLE IF NOT EXISTS merchant_api_logs (
  id BIGSERIAL PRIMARY KEY,
  merchant_id BIGINT DEFAULT 0,         -- 商户（鉴权失败且商户未知时为 0）
  app_key VARCHAR(64) DEFAULT '',       -- 请求头携带的 appKey（原样记录）
  method VARCHAR(8) DEFAULT '',
  path VARCHAR(255) DEFAULT '',         -- 请求路径（不含 query）
  ip VARCHAR(64) DEFAULT '',
  status_code INT DEFAULT 0,
  latency_ms INT DEFAULT 0,
  msg VARCHAR(255) DEFAULT '',          -- 失败原因摘要（成功为空）
  created_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_merchant_api_logs_merchant_id ON merchant_api_logs (merchant_id);
CREATE INDEX IF NOT EXISTS idx_merchant_api_logs_app_key ON merchant_api_logs (app_key);
CREATE INDEX IF NOT EXISTS idx_merchant_api_logs_created_at ON merchant_api_logs (created_at);

-- 租户表（name/code 唯一，软删留痕；存在关联应用用户时禁止删除）
CREATE TABLE IF NOT EXISTS tenants (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(64) UNIQUE NOT NULL,
  code VARCHAR(64) UNIQUE NOT NULL,     -- 租户编码（创建后不可改）
  contact_name VARCHAR(64) DEFAULT '',
  contact_phone VARCHAR(32) DEFAULT '',
  remark VARCHAR(255) DEFAULT '',
  status INT DEFAULT 1,                 -- 1 启用 0 禁用
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_tenants_deleted_at ON tenants (deleted_at);

-- 租户成员角色绑定表（租户端授权：平台 app_user × 平台租户 ID，内置双角色；
-- 绑定即授权、无软删，孤儿绑定在 AppAuth 闸门处天然失效）
CREATE TABLE IF NOT EXISTS tenant_user_roles (
  app_user_id BIGINT NOT NULL,
  tenant_platform_id BIGINT NOT NULL,   -- 平台租户 ID（X-Tenant-ID 同口径）
  role VARCHAR(32) NOT NULL,            -- tenant_admin | member
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  PRIMARY KEY (app_user_id, tenant_platform_id)
);
CREATE INDEX IF NOT EXISTS idx_tenant_user_roles_tenant ON tenant_user_roles (tenant_platform_id);

CREATE INDEX IF NOT EXISTS idx_app_user_tenants_deleted_at ON app_user_tenants (deleted_at);

-- MCP 服务器配置表（智能体工具源；token 只存 AES-GCM 密文，软删留痕；name/code 唯一）
CREATE TABLE IF NOT EXISTS mcp_servers (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(20) NOT NULL,
  code VARCHAR(64) NOT NULL,           -- 稳定引用（Agent 以 mcp:<code>:<tool> 绑定）
  transport VARCHAR(20) NOT NULL,      -- streamable_http | sse
  base_url VARCHAR(255) NOT NULL,
  headers TEXT,                        -- 自定义请求头 JSON 数组（明文非敏感头）
  token_enc VARCHAR(512),              -- AES-GCM 密文（base64），永不输出
  token_mask VARCHAR(32),              -- 展示掩码
  remark VARCHAR(200) DEFAULT '',
  status SMALLINT DEFAULT 1,           -- 1 启用 0 禁用
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_mcp_servers_name ON mcp_servers (name);
CREATE UNIQUE INDEX IF NOT EXISTS uk_mcp_servers_code ON mcp_servers (code);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_deleted_at ON mcp_servers (deleted_at);

-- 技能表（多文件技能包主表）
CREATE TABLE IF NOT EXISTS skills (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(20) NOT NULL,
  code VARCHAR(64) NOT NULL,           -- 稳定引用（Agent 以 code 绑定）
  description VARCHAR(200) DEFAULT '',
  instruction TEXT NOT NULL,           -- 主指令（SKILL.md 等价物，Markdown）
  remark VARCHAR(200) DEFAULT '',
  status SMALLINT DEFAULT 1,           -- 1 启用 0 禁用
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_skills_name ON skills (name);
CREATE UNIQUE INDEX IF NOT EXISTS uk_skills_code ON skills (code);
CREATE INDEX IF NOT EXISTS idx_skills_deleted_at ON skills (deleted_at);

-- 技能附属文件表（随技能整体替换，物理删插）
CREATE TABLE IF NOT EXISTS skill_files (
  id BIGSERIAL PRIMARY KEY,
  skill_id BIGINT NOT NULL,
  path VARCHAR(128) NOT NULL,          -- 相对路径
  content TEXT,
  file_size INT DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_skill_files_skill_id ON skill_files (skill_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_skill_file_path ON skill_files (skill_id, path);

-- ALTER TABLE agents ADD COLUMN skills VARCHAR(512); -- 由 AutoMigrate 自动完成，此处仅参考

-- 登录日志表（追加型流水：无软删，清空/保留期清理均为物理删除）
CREATE TABLE IF NOT EXISTS login_logs (
  id BIGSERIAL PRIMARY KEY,
  username VARCHAR(64) DEFAULT '',   -- 尝试登录的用户名（可能不存在）
  ip VARCHAR(64) DEFAULT '',
  user_agent VARCHAR(255) DEFAULT '',
  device VARCHAR(16) DEFAULT '',     -- web / app
  status SMALLINT DEFAULT 0,         -- 1 成功 0 失败
  msg VARCHAR(255) DEFAULT '',       -- 失败原因（成功为空）
  created_at TIMESTAMPTZ             -- 登录时间
);
CREATE INDEX IF NOT EXISTS idx_login_logs_username ON login_logs (username);
CREATE INDEX IF NOT EXISTS idx_login_logs_ip ON login_logs (ip);
CREATE INDEX IF NOT EXISTS idx_login_logs_status ON login_logs (status);
CREATE INDEX IF NOT EXISTS idx_login_logs_created_at ON login_logs (created_at);
