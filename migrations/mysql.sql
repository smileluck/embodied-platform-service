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
  UNIQUE KEY uk_name (name),
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
-- INSERT INTO roles (id, name, remark) VALUES (1, '超级管理员', '拥有全部权限');
-- INSERT INTO users (id, username, password, nickname, status) VALUES (1, 'admin', '$2a$10$...', '超级管理员', 1);
-- INSERT INTO user_roles VALUES (1, 1);

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
  name VARCHAR(255) DEFAULT '',         -- 展示名（兼作下载文件名，提交时已按语言翻译）
  locale VARCHAR(16) DEFAULT '',        -- 提交时语言快照（worker 表头/行内值翻译用）
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

-- 租户 RBAC 本地三表（2026-10-09 自平台下沉；tenant_id/user_id 为平台 ID，无外键引用，
-- 账号身份事实源在平台 tenant_users。旧本地表 tenant_user_roles（app_user 双角色时代）
-- 已于 2026-10-08 退役，由 migrateLegacy 启动幂等清理）
CREATE TABLE IF NOT EXISTS tenant_roles (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT UNSIGNED NOT NULL,   -- 平台租户 ID
  name VARCHAR(64) DEFAULT '',
  code VARCHAR(64) NOT NULL,            -- (tenant_id, code) 唯一；软删墓碑改写释放槽位
  remark VARCHAR(255) DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  UNIQUE KEY uk_tenant_roles_tenant_code (tenant_id, code),
  KEY idx_tenant_roles_deleted (deleted_at)
);
CREATE TABLE IF NOT EXISTS tenant_role_perms (
  role_id BIGINT UNSIGNED NOT NULL,
  perm_code VARCHAR(64) NOT NULL,       -- 权限码（本地注册表 permcatalog.go 目录内）
  created_at DATETIME,
  PRIMARY KEY (role_id, perm_code)
);
CREATE TABLE IF NOT EXISTS tenant_user_role_binds (
  user_id BIGINT UNSIGNED NOT NULL,     -- 平台 tenant_user ID
  role_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME,
  PRIMARY KEY (user_id, role_id)
);

-- 租户部门（2026-10-10 租户内部数据隔离基础能力：树形自引用 parent_id 0=根；
-- tenant_id 为平台租户 ID，(tenant_id, code) 唯一，软删墓碑改写 code 释放槽位）
CREATE TABLE IF NOT EXISTS tenant_depts (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  tenant_id BIGINT UNSIGNED NOT NULL,   -- 平台租户 ID
  parent_id BIGINT UNSIGNED NOT NULL DEFAULT 0,  -- 父部门 ID，0=根
  name VARCHAR(64) DEFAULT '',
  code VARCHAR(64) NOT NULL,            -- (tenant_id, code) 唯一；软删墓碑改写释放槽位
  sort INT NOT NULL DEFAULT 0,
  remark VARCHAR(255) DEFAULT '',
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  UNIQUE KEY uk_tenant_depts_tenant_code (tenant_id, code),
  KEY idx_tenant_depts_parent (parent_id),
  KEY idx_tenant_depts_deleted (deleted_at)
);
CREATE TABLE IF NOT EXISTS tenant_user_dept_binds (
  user_id BIGINT UNSIGNED NOT NULL,     -- 平台 tenant_user ID
  dept_id BIGINT UNSIGNED NOT NULL,
  created_at DATETIME,
  PRIMARY KEY (user_id, dept_id)        -- 多部门归属，替换式物理删除
);

-- MCP 服务器配置表（智能体工具源；token 只存 AES-GCM 密文，软删留痕；name/code 唯一）
CREATE TABLE IF NOT EXISTS `mcp_servers` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(20) NOT NULL,
  `code` VARCHAR(64) NOT NULL COMMENT '稳定引用（Agent 以 mcp:<code>:<tool> 绑定）',
  `transport` VARCHAR(20) NOT NULL COMMENT 'streamable_http | sse',
  `base_url` VARCHAR(255) NOT NULL,
  `headers` TEXT COMMENT '自定义请求头 JSON 数组（明文非敏感头）',
  `token_enc` VARCHAR(512) COMMENT 'AES-GCM 密文（base64），永不输出',
  `token_mask` VARCHAR(32) COMMENT '展示掩码',
  `remark` VARCHAR(200) DEFAULT '',
  `status` TINYINT DEFAULT 1 COMMENT '1 启用 0 禁用',
  `created_at` DATETIME(3),
  `updated_at` DATETIME(3),
  `deleted_at` DATETIME(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_mcp_servers_name` (`name`),
  UNIQUE KEY `uk_mcp_servers_code` (`code`),
  KEY `idx_mcp_servers_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- 技能表（多文件技能包主表）
CREATE TABLE IF NOT EXISTS `skills` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(20) NOT NULL,
  `code` VARCHAR(64) NOT NULL COMMENT '稳定引用（Agent 以 code 绑定）',
  `description` VARCHAR(200) DEFAULT '',
  `instruction` TEXT NOT NULL COMMENT '主指令（SKILL.md 等价物，Markdown）',
  `remark` VARCHAR(200) DEFAULT '',
  `status` TINYINT DEFAULT 1 COMMENT '1 启用 0 禁用',
  `created_at` DATETIME(3),
  `updated_at` DATETIME(3),
  `deleted_at` DATETIME(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_skills_name` (`name`),
  UNIQUE KEY `uk_skills_code` (`code`),
  KEY `idx_skills_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- 技能附属文件表（随技能整体替换，物理删插）
CREATE TABLE IF NOT EXISTS `skill_files` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `skill_id` BIGINT UNSIGNED NOT NULL,
  `path` VARCHAR(128) NOT NULL COMMENT '相对路径',
  `content` TEXT,
  `file_size` INT DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_skill_files_skill_id` (`skill_id`),
  UNIQUE KEY `uk_skill_file_path` (`skill_id`, `path`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ALTER TABLE `agents` ADD COLUMN `skills` VARCHAR(512); -- 由 AutoMigrate 自动完成，此处仅参考

-- 登录日志表（追加型流水：无软删，清空/保留期清理均为物理删除）
CREATE TABLE IF NOT EXISTS `login_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(64) DEFAULT '' COMMENT '尝试登录的用户名（可能不存在）',
  `ip` VARCHAR(64) DEFAULT '',
  `user_agent` VARCHAR(255) DEFAULT '',
  `device` VARCHAR(16) DEFAULT '' COMMENT 'web / app',
  `status` TINYINT DEFAULT 0 COMMENT '1 成功 0 失败',
  `msg` VARCHAR(255) DEFAULT '' COMMENT '失败原因（成功为空）',
  `created_at` DATETIME(3) COMMENT '登录时间',
  PRIMARY KEY (`id`),
  KEY `idx_login_logs_username` (`username`),
  KEY `idx_login_logs_ip` (`ip`),
  KEY `idx_login_logs_status` (`status`),
  KEY `idx_login_logs_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- 租户门户登录日志表（独立流水：带平台租户 ID，门户查询按 tid 锁定）
CREATE TABLE IF NOT EXISTS `tenant_login_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED DEFAULT 0 COMMENT '平台租户 ID（自省失败落库时为 0）',
  `username` VARCHAR(64) DEFAULT '' COMMENT '尝试登录的用户名（可能不存在）',
  `ip` VARCHAR(64) DEFAULT '',
  `user_agent` VARCHAR(255) DEFAULT '',
  `status` TINYINT DEFAULT 0 COMMENT '1 成功 0 失败',
  `msg` VARCHAR(255) DEFAULT '' COMMENT '失败原因（成功为空）',
  `created_at` DATETIME(3) COMMENT '登录时间',
  PRIMARY KEY (`id`),
  KEY `idx_tenant_login_logs_tenant_id` (`tenant_id`),
  KEY `idx_tenant_login_logs_username` (`username`),
  KEY `idx_tenant_login_logs_status` (`status`),
  KEY `idx_tenant_login_logs_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- 租户门户操作日志表（写请求审计流水：带平台租户 ID）
CREATE TABLE IF NOT EXISTS `tenant_operation_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED DEFAULT 0 COMMENT '平台租户 ID',
  `user_id` BIGINT UNSIGNED DEFAULT 0 COMMENT '操作人（平台 tenant_user ID）',
  `username` VARCHAR(64) DEFAULT '' COMMENT '操作人用户名快照',
  `method` VARCHAR(8) DEFAULT '',
  `path` VARCHAR(255) DEFAULT '' COMMENT '实际请求路径（含资源 ID 与 query）',
  `route` VARCHAR(128) DEFAULT '' COMMENT '路由模板',
  `action` VARCHAR(64) DEFAULT '' COMMENT '中文动作名',
  `params` TEXT COMMENT '请求参数摘要（敏感字段脱敏、超长截断）',
  `ip` VARCHAR(64) DEFAULT '',
  `user_agent` VARCHAR(255) DEFAULT '',
  `status_code` INT DEFAULT 0 COMMENT '响应状态码',
  `latency_ms` INT DEFAULT 0 COMMENT '耗时（毫秒）',
  `created_at` DATETIME(3) COMMENT '操作时间',
  PRIMARY KEY (`id`),
  KEY `idx_tenant_operation_logs_tenant_id` (`tenant_id`),
  KEY `idx_tenant_operation_logs_username` (`username`),
  KEY `idx_tenant_operation_logs_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
