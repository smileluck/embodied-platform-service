-- SQLite 建表参考（本地开发/测试；种子数据由程序启动时写入）
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  password TEXT NOT NULL,
  nickname TEXT DEFAULT '',
  phone TEXT DEFAULT '',
  email TEXT DEFAULT '',
  status INTEGER DEFAULT 1,
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS roles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  remark TEXT DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS permissions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  type TEXT DEFAULT 'menu', -- menu 菜单 | button 按钮权限（api 已废弃，启动迁移自动转为 button）
  method TEXT DEFAULT '',
  path TEXT DEFAULT '',
  parent_id INTEGER DEFAULT 0,
  icon TEXT DEFAULT '',
  sort INTEGER DEFAULT 0,
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS user_roles (
  user_id INTEGER NOT NULL,
  role_id INTEGER NOT NULL,
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id INTEGER NOT NULL,
  permission_id INTEGER NOT NULL,
  PRIMARY KEY (role_id, permission_id)
);

-- 文件元数据表（对象本体在 driver 对应的存储后端）
CREATE TABLE IF NOT EXISTS files (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  driver TEXT NOT NULL,                 -- local | oss | cos | tos | minio（落库时的存储后端）
  object_key TEXT NOT NULL UNIQUE,      -- 服务端生成的对象 key
  name TEXT NOT NULL,                   -- 原始文件名
  ext TEXT DEFAULT '',
  size INTEGER DEFAULT 0,
  content_type TEXT DEFAULT '',
  uploader_id INTEGER DEFAULT 0,
  uploader_name TEXT DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_files_driver ON files (driver);
CREATE INDEX IF NOT EXISTS idx_files_ext ON files (ext);
CREATE INDEX IF NOT EXISTS idx_files_uploader_id ON files (uploader_id);
CREATE INDEX IF NOT EXISTS idx_files_deleted_at ON files (deleted_at);

-- 异步导出任务记录表（产物本体在 driver 对应的存储后端；无软删，保留期清理为物理删除）
CREATE TABLE IF NOT EXISTS export_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER DEFAULT 0,            -- 任务归属用户
  biz TEXT DEFAULT '',                  -- 业务类型（user / login_log / op_log）
  name TEXT DEFAULT '',                 -- 展示名（兼作下载文件名）
  params TEXT,                          -- 查询条件快照（JSON）
  driver TEXT DEFAULT '',               -- 产物落库时的存储后端
  object_key TEXT DEFAULT '',           -- 产物对象 key
  size INTEGER DEFAULT 0,               -- 产物字节数（含 BOM）
  rows INTEGER DEFAULT 0,               -- 已导出数据行数（不含表头）
  status TEXT DEFAULT 'pending',        -- pending | running | done | failed
  truncated INTEGER DEFAULT 0,          -- 触及大小/行数上限被截断
  error TEXT DEFAULT '',                -- 失败原因（成功为空）
  created_at DATETIME,
  finished_at DATETIME                  -- 完成/失败时间（未结束为 NULL）
);
CREATE INDEX IF NOT EXISTS idx_export_records_user_id ON export_records (user_id);
CREATE INDEX IF NOT EXISTS idx_export_records_status ON export_records (status);
CREATE INDEX IF NOT EXISTS idx_export_records_created_at ON export_records (created_at);

-- IP 黑名单表（管理员手工维护的持久化封禁；软删即解封留痕）
CREATE TABLE IF NOT EXISTS ip_blacklist (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ip TEXT NOT NULL UNIQUE,              -- 单个 IP（不支持 CIDR）
  reason TEXT DEFAULT '',               -- 封禁原因
  source TEXT NOT NULL DEFAULT 'manual', -- manual | auto（登录连续失败自动封禁）
  expire_at DATETIME,                   -- 过期时间（NULL 为永久封禁）
  creator_id INTEGER DEFAULT 0,
  creator_name TEXT DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_ip_blacklist_deleted_at ON ip_blacklist (deleted_at);

-- 商户表（开放 API 授权；app_secret 只存哈希 SHA-256(AppKey + ":" + secret)，软删留痕）
CREATE TABLE IF NOT EXISTS merchants (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  code TEXT NOT NULL,                   -- 商户编码（创建后不可改）
  app_key TEXT NOT NULL,                -- 开放 API 调用凭证 key（mk_ 前缀）
  app_secret_hash TEXT NOT NULL,        -- secret 哈希，明文仅创建/重置时返回一次
  contact_name TEXT DEFAULT '',
  contact_phone TEXT DEFAULT '',
  contact_email TEXT DEFAULT '',
  status INTEGER DEFAULT 1,             -- 1 启用 2 禁用
  remark TEXT DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_merchants_code ON merchants (code);
CREATE UNIQUE INDEX IF NOT EXISTS uk_merchants_app_key ON merchants (app_key);
CREATE INDEX IF NOT EXISTS idx_merchants_deleted_at ON merchants (deleted_at);

-- 开放 API 调用日志表（无软删，保留期清理为物理删除）
CREATE TABLE IF NOT EXISTS merchant_api_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  merchant_id INTEGER DEFAULT 0,        -- 商户（鉴权失败且商户未知时为 0）
  app_key TEXT DEFAULT '',              -- 请求头携带的 appKey（原样记录）
  method TEXT DEFAULT '',
  path TEXT DEFAULT '',                 -- 请求路径（不含 query）
  ip TEXT DEFAULT '',
  status_code INTEGER DEFAULT 0,
  latency_ms INTEGER DEFAULT 0,
  msg TEXT DEFAULT '',                  -- 失败原因摘要（成功为空）
  created_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_merchant_api_logs_merchant_id ON merchant_api_logs (merchant_id);
CREATE INDEX IF NOT EXISTS idx_merchant_api_logs_app_key ON merchant_api_logs (app_key);
CREATE INDEX IF NOT EXISTS idx_merchant_api_logs_created_at ON merchant_api_logs (created_at);

-- 租户表（name/code 唯一，软删留痕；存在关联应用用户时禁止删除）
CREATE TABLE IF NOT EXISTS tenants (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  code TEXT NOT NULL,                   -- 租户编码（创建后不可改）
  contact_name TEXT DEFAULT '',
  contact_phone TEXT DEFAULT '',
  remark TEXT DEFAULT '',
  status INTEGER DEFAULT 1,             -- 1 启用 0 禁用
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenants_name ON tenants (name);
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenants_code ON tenants (code);
CREATE INDEX IF NOT EXISTS idx_tenants_deleted_at ON tenants (deleted_at);

-- 应用用户表（多租户终端用户；username 唯一，密码只存 bcrypt 哈希，软删留痕）
CREATE TABLE IF NOT EXISTS app_users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL,
  password_hash TEXT NOT NULL,          -- bcrypt 哈希，永不输出
  nickname TEXT DEFAULT '',
  phone TEXT DEFAULT '',
  email TEXT DEFAULT '',
  status INTEGER DEFAULT 1,             -- 1 启用 0 禁用
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_app_users_username ON app_users (username);
CREATE INDEX IF NOT EXISTS idx_app_users_deleted_at ON app_users (deleted_at);

-- 应用用户-租户关联表（复合唯一索引；替换关联时物理删除）
CREATE TABLE IF NOT EXISTS app_user_tenants (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  app_user_id INTEGER NOT NULL,
  tenant_id INTEGER NOT NULL,
  created_at DATETIME,
  deleted_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_app_user_tenant ON app_user_tenants (app_user_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_app_user_tenants_tenant_id ON app_user_tenants (tenant_id);
CREATE INDEX IF NOT EXISTS idx_app_user_tenants_deleted_at ON app_user_tenants (deleted_at);
