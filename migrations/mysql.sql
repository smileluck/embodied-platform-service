-- MySQL 建表 + 种子数据（程序 AutoMigrate 之外的参考脚本）
-- 默认超级管理员：admin / 123456（上线前务必修改）
CREATE DATABASE IF NOT EXISTS smilex_admin DEFAULT CHARSET utf8mb4;
USE smilex_admin;

CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(64) NOT NULL,
  password VARCHAR(128) NOT NULL,
  nickname VARCHAR(64) DEFAULT '',
  phone VARCHAR(32) DEFAULT '',
  email VARCHAR(128) DEFAULT '',
  status INT DEFAULT 1,
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  UNIQUE KEY uk_username (username),
  KEY idx_deleted (deleted_at)
);

CREATE TABLE IF NOT EXISTS roles (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,

  remark VARCHAR(255) DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  KEY idx_deleted (deleted_at)
);

CREATE TABLE IF NOT EXISTS permissions (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,

  type VARCHAR(16) DEFAULT 'menu', -- menu 菜单 | button 按钮权限（api 已废弃，启动迁移自动转为 button）
  method VARCHAR(16) DEFAULT '',
  path VARCHAR(255) DEFAULT '',
  parent_id BIGINT UNSIGNED DEFAULT 0,
  icon VARCHAR(512) DEFAULT '',
  sort INT DEFAULT 0,
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  KEY idx_deleted (deleted_at)
);

CREATE TABLE IF NOT EXISTS user_roles (
  user_id BIGINT UNSIGNED NOT NULL,
  role_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id BIGINT UNSIGNED NOT NULL,
  permission_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (role_id, permission_id)
);

-- 种子数据（密码哈希由程序启动时自动补全，此处的哈希对应 123456）
-- INSERT INTO permissions (id, name, code, type, method, path) VALUES (1, '全部权限', 'all', 'api', '*', '*');
-- INSERT INTO roles (id, name, remark) VALUES (1, '超级管理员', '拥有全部权限');
-- INSERT INTO users (id, username, password, nickname, status) VALUES (1, 'admin', '$2a$10$...', '超级管理员', 1);
-- INSERT INTO user_roles VALUES (1, 1);
-- INSERT INTO role_permissions VALUES (1, 1);

-- 文件元数据表（对象本体在 driver 对应的存储后端）
CREATE TABLE IF NOT EXISTS files (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  driver VARCHAR(16) NOT NULL,          -- local | oss | cos | tos | minio（落库时的存储后端）
  object_key VARCHAR(512) NOT NULL,     -- 服务端生成的对象 key
  name VARCHAR(255) NOT NULL,           -- 原始文件名
  ext VARCHAR(16) DEFAULT '',
  size BIGINT DEFAULT 0,
  content_type VARCHAR(128) DEFAULT '',
  uploader_id BIGINT UNSIGNED DEFAULT 0,
  uploader_name VARCHAR(64) DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  UNIQUE KEY uk_object_key (object_key),
  KEY idx_driver (driver),
  KEY idx_ext (ext),
  KEY idx_uploader_id (uploader_id),
  KEY idx_deleted (deleted_at)
);

-- 异步导出任务记录表（产物本体在 driver 对应的存储后端；无软删，保留期清理为物理删除）
CREATE TABLE IF NOT EXISTS export_records (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT UNSIGNED DEFAULT 0,    -- 任务归属用户
  biz VARCHAR(32) DEFAULT '',           -- 业务类型（user / login_log / op_log）
  name VARCHAR(255) DEFAULT '',         -- 展示名（兼作下载文件名）
  params TEXT,                          -- 查询条件快照（JSON）
  driver VARCHAR(16) DEFAULT '',        -- 产物落库时的存储后端
  object_key VARCHAR(512) DEFAULT '',   -- 产物对象 key
  size BIGINT DEFAULT 0,                -- 产物字节数（含 BOM）
  rows INT DEFAULT 0,                   -- 已导出数据行数（不含表头）
  status VARCHAR(16) DEFAULT 'pending', -- pending | running | done | failed
  truncated TINYINT(1) DEFAULT 0,       -- 触及大小/行数上限被截断
  error VARCHAR(512) DEFAULT '',        -- 失败原因（成功为空）
  created_at DATETIME,
  finished_at DATETIME,                 -- 完成/失败时间（未结束为 NULL）
  KEY idx_user_id (user_id),
  KEY idx_status (status),
  KEY idx_created_at (created_at)
);

-- IP 黑名单表（管理员手工维护的持久化封禁；软删即解封留痕）
CREATE TABLE IF NOT EXISTS ip_blacklist (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  ip VARCHAR(64) NOT NULL,              -- 单个 IP（不支持 CIDR）
  reason VARCHAR(255) DEFAULT '',       -- 封禁原因
  source VARCHAR(16) NOT NULL DEFAULT 'manual', -- manual | auto（登录连续失败自动封禁）
  expire_at DATETIME,                   -- 过期时间（NULL 为永久封禁）
  creator_id BIGINT UNSIGNED DEFAULT 0,
  creator_name VARCHAR(64) DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  UNIQUE KEY uk_ip (ip),
  KEY idx_deleted (deleted_at)
);

-- 商户表（开放 API 授权；app_secret 只存哈希 SHA-256(AppKey + ":" + secret)，软删留痕）
CREATE TABLE IF NOT EXISTS merchants (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  code VARCHAR(64) NOT NULL,            -- 商户编码（创建后不可改）
  app_key VARCHAR(64) NOT NULL,         -- 开放 API 调用凭证 key（mk_ 前缀）
  app_secret_hash VARCHAR(128) NOT NULL, -- secret 哈希，明文仅创建/重置时返回一次
  contact_name VARCHAR(64) DEFAULT '',
  contact_phone VARCHAR(32) DEFAULT '',
  contact_email VARCHAR(128) DEFAULT '',
  status INT DEFAULT 1,                 -- 1 启用 2 禁用
  remark VARCHAR(255) DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  UNIQUE KEY uk_code (code),
  UNIQUE KEY uk_app_key (app_key),
  KEY idx_deleted (deleted_at)
);

-- 开放 API 调用日志表（无软删，保留期清理为物理删除）
CREATE TABLE IF NOT EXISTS merchant_api_logs (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  merchant_id BIGINT UNSIGNED DEFAULT 0, -- 商户（鉴权失败且商户未知时为 0）
  app_key VARCHAR(64) DEFAULT '',       -- 请求头携带的 appKey（原样记录）
  method VARCHAR(8) DEFAULT '',
  path VARCHAR(255) DEFAULT '',         -- 请求路径（不含 query）
  ip VARCHAR(64) DEFAULT '',
  status_code INT DEFAULT 0,
  latency_ms INT DEFAULT 0,
  msg VARCHAR(255) DEFAULT '',          -- 失败原因摘要（成功为空）
  created_at DATETIME,
  KEY idx_merchant_id (merchant_id),
  KEY idx_app_key (app_key),
  KEY idx_created_at (created_at)
);

-- 租户表（name/code 唯一，软删留痕；存在关联应用用户时禁止删除）
CREATE TABLE IF NOT EXISTS tenants (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  code VARCHAR(64) NOT NULL,            -- 租户编码（创建后不可改）
  contact_name VARCHAR(64) DEFAULT '',
  contact_phone VARCHAR(32) DEFAULT '',
  remark VARCHAR(255) DEFAULT '',
  status INT DEFAULT 1,                 -- 1 启用 0 禁用
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  UNIQUE KEY uk_name (name),
  UNIQUE KEY uk_code (code),
  KEY idx_deleted (deleted_at)
);

-- 应用用户表（多租户终端用户；username 唯一，密码只存 bcrypt 哈希，软删留痕）
CREATE TABLE IF NOT EXISTS app_users (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(64) NOT NULL,
  password_hash VARCHAR(128) NOT NULL,  -- bcrypt 哈希，永不输出
  nickname VARCHAR(64) DEFAULT '',
  phone VARCHAR(32) DEFAULT '',
  email VARCHAR(128) DEFAULT '',
  status INT DEFAULT 1,                 -- 1 启用 0 禁用
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  UNIQUE KEY uk_username (username),
  KEY idx_deleted (deleted_at)
);

-- 应用用户-租户关联表（复合唯一索引；替换关联时物理删除）
CREATE TABLE IF NOT EXISTS app_user_tenants (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  app_user_id BIGINT UNSIGNED NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME,
  deleted_at DATETIME,
  UNIQUE KEY uk_app_user_tenant (app_user_id, tenant_id),
  KEY idx_tenant_id (tenant_id),
  KEY idx_deleted (deleted_at)
);
