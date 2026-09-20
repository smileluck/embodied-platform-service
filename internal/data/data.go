// Package data 基础设施层：多数据库工厂、AutoMigrate 与种子数据。
package data

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"
	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	"github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Data 持久化入口（对应 Kratos 的 Data struct）
type Data struct {
	DB *gorm.DB
}

// NewData 按 config.db.driver 创建对应数据库连接
func NewData(c *conf.Bootstrap) (*Data, func(), error) {
	var (
		db  *gorm.DB
		err error
	)
	gormCfg := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	}
	switch c.DB.Driver {
	case "mysql":
		ensureMySQLDatabase(c.DB.MySQL)
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			c.DB.MySQL.User, c.DB.MySQL.Password, c.DB.MySQL.Host, c.DB.MySQL.Port, c.DB.MySQL.DBName, c.DB.MySQL.Charset)
		db, err = gorm.Open(mysql.Open(dsn), gormCfg)
	case "postgres":
		ensurePostgresDatabase(c.DB.Postgres)
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.DB.Postgres.Host, c.DB.Postgres.Port, c.DB.Postgres.User, c.DB.Postgres.Password, c.DB.Postgres.DBName, c.DB.Postgres.SSLMode)
		db, err = gorm.Open(postgres.Open(dsn), gormCfg)
	case "sqlite":
		_ = os.MkdirAll(filepath.Dir(c.DB.SQLite.Path), 0o755)
		db, err = gorm.Open(sqlite.Open(c.DB.SQLite.Path), gormCfg)
	default:
		return nil, nil, fmt.Errorf("unsupported db driver: %s", c.DB.Driver)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", c.DB.Driver, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	maxOpen, maxIdle := 20, 10
	switch c.DB.Driver {
	case "mysql":
		maxOpen, maxIdle = c.DB.MySQL.MaxOpenConns, c.DB.MySQL.MaxIdleConns
	case "postgres":
		maxOpen, maxIdle = c.DB.Postgres.MaxOpenConns, c.DB.Postgres.MaxIdleConns
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)

	d := &Data{DB: db}
	if c.DB.AutoMigrate {
		if err := d.migrateAndSeed(); err != nil {
			return nil, nil, err
		}
	}
	cleanup := func() { _ = sqlDB.Close() }
	return d, cleanup, nil
}

// ensureMySQLDatabase 库不存在时自动创建（连接 information_schema 建库后再正常连接）
func ensureMySQLDatabase(c conf.MySQL) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Charset)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET %s", c.DBName, c.Charset)); err != nil {
		logger.Warn("auto create mysql database failed", zap.Error(err))
	}
}

// ensurePostgresDatabase 库不存在时自动创建（先连 postgres 库执行 CREATE DATABASE）
func ensurePostgresDatabase(c conf.Postgres) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.SSLMode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(fmt.Sprintf(`SELECT 'CREATE DATABASE %s' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '%s')`, c.DBName, c.DBName)); err != nil {
		return
	}
	if _, err := sqlDB.Exec("CREATE DATABASE " + c.DBName); err != nil {
		logger.Warn("auto create postgres database failed", zap.Error(err))
	}
}

// migrateAndSeed 自动建表 + 存量迁移 + 种子数据。
// 平台是唯一身份源：本地不种子任何账号/超管角色——准入唯一来源是平台商户成员绑定
// （/users/sync 拉取建投影），内置「商户管理员」角色由平台的商户管理员标记驱动绑定
// （见 biz/admission）。冷启动：平台把某成员设为商户管理员后，其首次请求即自动准入。
func (d *Data) migrateAndSeed() error {
	if err := d.DB.AutoMigrate(
		&model.PlatformUserPO{}, &model.PlatformUserRolePO{},
		&model.RolePO{}, &model.PermissionPO{}, &model.RolePermissionPO{},
		&model.OperationLogPO{},
		&model.FilePO{}, &model.ExportRecordPO{}, &model.IPBlacklistPO{},
		&model.TenantPO{}, &model.AppUserPO{}, &model.AppUserTenantPO{},
		&model.AgentProviderPO{}, &model.AgentModelPO{}, &model.AgentPO{},
		&model.AgentConversationPO{}, &model.AgentConversationMsgPO{}, &model.AgentUsageLogPO{}, &model.DictTypePO{}, &model.DictItemPO{}, &model.SysConfigPO{}, &model.NoticePO{}, &model.NoticeReadPO{},
	); err != nil {
		return err
	}

	if err := d.migrateLegacy(); err != nil {
		return err
	}

	// 商户管理员内置角色（须先于菜单/按钮补齐存在，绑定在两个 ensure 循环中一并落到角色2）
	if err := d.ensureMerchantAdminRole(); err != nil {
		return err
	}

	// 系统菜单幂等补齐（存量库升级路径），需先于按钮补齐执行以解析按钮归属
	if err := d.ensureSystemMenus(); err != nil {
		return err
	}
	// 系统管理接口权限点补齐并绑定商户管理员角色（存量库/全新库统一走此路径，幂等）
	return d.ensureSystemButtonPerms()
}

// systemMenuDef 系统菜单幂等定义（ParentCode 为父菜单 code，缺失时落为顶级菜单；Type 缺省为 menu）
type systemMenuDef struct {
	Name       string
	Code       string
	Type       string // dir | menu（缺省 menu）
	Path       string
	Icon       string
	Sort       int
	ParentCode string
}

// systemMenus 需幂等保障的系统菜单清单（按 code 判断存在性；不指定固定 ID，避免与存量库自增记录冲突）
var systemMenus = []systemMenuDef{
	{Name: "首页", Code: "menu:dashboard", Path: "/dashboard", Icon: "HomeOutline", Sort: 1},
	{Name: "系统管理", Code: "menu:system", Type: "dir", Icon: "SettingsOutline", Sort: 2},
	{Name: "用户准入", Code: "menu:user", Path: "/system/users", ParentCode: "menu:system", Icon: "PersonOutline", Sort: 1},
	{Name: "角色管理", Code: "menu:role", Path: "/system/roles", ParentCode: "menu:system", Icon: "IdCardOutline", Sort: 2},
	{Name: "菜单管理", Code: "menu:menu", Path: "/system/menus", ParentCode: "menu:system", Icon: "MenuOutline", Sort: 4},
	{Name: "IP黑名单", Code: "menu:blacklist", Path: "/system/blacklist", ParentCode: "menu:system", Icon: "BanOutline", Sort: 6},
	// 设备中心（顶级目录分组，父级先于子菜单声明以解析 ParentCode）
	{Name: "设备中心", Code: "menu:deviceCenter", Type: "dir", Icon: "HardwareChipOutline", Sort: 3},
	{Name: "设备管理", Code: "menu:device", Path: "/device/devices", Icon: "HardwareChipOutline", Sort: 1, ParentCode: "menu:deviceCenter"},
	{Name: "型号管理", Code: "menu:deviceModel", Path: "/device/models", Icon: "CubeOutline", Sort: 2, ParentCode: "menu:deviceCenter"},
	// 租户中心（顶级目录分组）
	{Name: "租户中心", Code: "menu:tenantCenter", Type: "dir", Icon: "BusinessOutline", Sort: 4},
	{Name: "数据字典", Code: "menu:dict", Path: "/system/dicts", Icon: "BookOutline", Sort: 10, ParentCode: "menu:system"},
	{Name: "系统参数", Code: "menu:sysConfig", Path: "/system/configs", Icon: "SettingsOutline", Sort: 11, ParentCode: "menu:system"},
	{Name: "通知公告", Code: "menu:notice", Path: "/system/notices", Icon: "MegaphoneOutline", Sort: 12, ParentCode: "menu:system"},
	{Name: "关于我们", Code: "menu:about", Path: "/about", Icon: "InformationCircleOutline", Sort: 9},
	{Name: "日志管理", Code: "menu:log", Type: "dir", Icon: "DocumentTextOutline", Sort: 3},
	{Name: "操作日志", Code: "menu:opLog", Path: "/log/operation-logs", Icon: "ClipboardOutline", Sort: 2, ParentCode: "menu:log"},
	{Name: "文件管理", Code: "menu:file", Path: "/file", Icon: "FolderOpenOutline", Sort: 4},
	{Name: "IP黑名单", Code: "menu:blacklist", Path: "/system/blacklist", Icon: "BanOutline", Sort: 6, ParentCode: "menu:system"},
	{Name: "服务器监控", Code: "menu:monitor", Path: "/system/monitor", Icon: "SpeedometerOutline", Sort: 6},
	{Name: "租户中心", Code: "menu:tenantCenter", Type: "dir", Icon: "BusinessOutline", Sort: 2},
	{Name: "租户管理", Code: "menu:tenant", Path: "/tenant/tenants", Icon: "BusinessOutline", Sort: 1, ParentCode: "menu:tenantCenter"},
	{Name: "应用用户", Code: "menu:appUser", Path: "/tenant/app-users", Icon: "PeopleOutline", Sort: 2, ParentCode: "menu:tenantCenter"},
	// 智能体（LLM 配置底座，顶级目录分组，父级先于子菜单声明以解析 ParentCode）
	{Name: "智能体", Code: "menu:agent", Type: "dir", Icon: "SparklesOutline", Sort: 8},
	{Name: "模型供应商", Code: "menu:agentProvider", Path: "/agent/providers", Icon: "ServerOutline", Sort: 1, ParentCode: "menu:agent"},
	{Name: "Agent 配置", Code: "menu:agentList", Path: "/agent/agents", Icon: "ChatbubblesOutline", Sort: 2, ParentCode: "menu:agent"},
	{Name: "聊天测试", Code: "menu:agentChat", Path: "/agent/chat", Icon: "ChatboxEllipsesOutline", Sort: 3, ParentCode: "menu:agent"},
	{Name: "用量统计", Code: "menu:agentUsage", Path: "/agent/usage", Icon: "StatsChartOutline", Sort: 4, ParentCode: "menu:agent"},
	// 日志管理（顶级目录分组）
	{Name: "日志管理", Code: "menu:log", Type: "dir", Icon: "DocumentTextOutline", Sort: 5},
	{Name: "操作日志", Code: "menu:opLog", Path: "/log/operation-logs", Icon: "ClipboardOutline", Sort: 2, ParentCode: "menu:log"},
	// 文件管理（顶级菜单）
	{Name: "文件管理", Code: "menu:file", Path: "/file", Icon: "FolderOpenOutline", Sort: 6},
	// 服务器状态监控（顶级菜单）
	{Name: "服务器监控", Code: "menu:monitor", Path: "/system/monitor", Icon: "SpeedometerOutline", Sort: 7},
	// 关于我们（顶级菜单）
	{Name: "关于我们", Code: "menu:about", Path: "/about", Icon: "InformationCircleOutline", Sort: 9},
}

// ensureMerchantAdminRole 商户管理员内置角色（固定 ID=2，幂等）：
// 由平台侧商户成员的商户管理员标记驱动绑定/解绑（biz/admission 中间件自愈 + 同步对账），
// 权限 = 全部种子菜单/按钮的显式绑定（非通配 all，可审计、后续可收敛），与超管角色同等锁定。
// 防御：ID=2 已被非本语义角色占用（老库自建角色）时记错误并禁用其绑定，避免误赋权。
func (d *Data) ensureMerchantAdminRole() error {
	const expectedName = "商户管理员"
	var po model.RolePO
	err := d.DB.First(&po, bizadmission.MerchantAdminRoleID).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if err := d.DB.Create(&model.RolePO{
			ID: bizadmission.MerchantAdminRoleID, Name: expectedName,
			Remark: "平台商户管理员标记驱动（内置，禁止手工分配）",
		}).Error; err != nil {
			return err
		}
		logger.Info("seeded merchant admin role (平台商户成员管理员标记驱动)")
		merchantRoleBound = true
		return nil
	case err != nil:
		return err
	}
	if po.Name != expectedName {
		// 非「商户管理员」语义的存量角色占了 ID=2：不接管、不绑定（管理员投影功能停用）
		logger.Error("role id 2 occupied by foreign role, merchant admin role disabled",
			zap.String("name", po.Name))
		merchantRoleBound = false
		return nil
	}
	merchantRoleBound = true
	return nil
}

// merchantRoleBound 商户管理员角色（ID=2）是否可用（种子语义未被占用）；
// 不可用时所有角色2绑定与投影逻辑跳过（避免把全部权限绑给无关角色）
var merchantRoleBound bool

// ensureSystemMenus 幂等补齐系统菜单并绑定商户管理员角色（每次启动执行）：
// 按 code 查找（type 兼容 menu/dir，存量迁移会翻转类型），缺失则插入（父级按 code 解析，缺失时落为顶级菜单）
func (d *Data) ensureSystemMenus() error {
	for _, m := range systemMenus {
		var po model.PermissionPO
		err := d.DB.Unscoped().Where("code = ? AND type IN ?", m.Code, []string{string(permission.TypeDir), string(permission.TypeMenu)}).First(&po).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			t := m.Type
			if t == "" {
				t = string(permission.TypeMenu)
			}
			po = model.PermissionPO{Name: m.Name, Code: m.Code, Type: t, Path: m.Path, Icon: m.Icon, Sort: m.Sort}
			// 父级菜单按 code 解析（存量库菜单 ID 与种子不保证一致；父级为 dir 或 menu 均可匹配）
			if m.ParentCode != "" {
				var parent model.PermissionPO
				if err := d.DB.Where("code = ? AND type IN ?", m.ParentCode, []string{string(permission.TypeDir), string(permission.TypeMenu)}).First(&parent).Error; err == nil {
					po.ParentID = parent.ID
				}
			}
			if err := d.DB.Create(&po).Error; err != nil {
				return err
			}
			logger.Info("ensured system menu", zap.String("code", m.Code))
		}
		// 商户管理员角色随种子同步持有全部菜单（可用时）
		if merchantRoleBound {
			if err := d.bindRole(bizadmission.MerchantAdminRoleID, po.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// bindRole 将权限绑定到指定角色（幂等；商户管理员为实际生效权限的唯一内置角色）
func (d *Data) bindRole(roleID, permID uint) error {
	var cnt int64
	if err := d.DB.Model(&model.RolePermissionPO{}).
		Where("role_id = ? AND permission_id = ?", roleID, permID).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	return d.DB.Create(&model.RolePermissionPO{RoleID: roleID, PermissionID: permID}).Error
}

// systemButtonPermDef 系统管理接口权限点定义（code 为幂等键，menu 为所属菜单 code）
type systemButtonPermDef struct {
	Name   string
	Code   string
	Menu   string
	Method string
	Path   string
	Sort   int
}

// systemButtonPerms 系统管理各接口的 button 权限点清单：
// code 控前端按钮显隐，method/path 绑定接口参与后端 RBAC 校验（path 支持中间通配 *）
var systemButtonPerms = []systemButtonPermDef{
	// 用户准入（成员来源=平台开放面本商户绑定集：新增/删除推送平台）
	{Name: "查询用户", Code: "user:list", Menu: "menu:user", Method: "GET", Path: "/api/v1/users", Sort: 1},
	{Name: "用户详情", Code: "user:view", Menu: "menu:user", Method: "GET", Path: "/api/v1/users/*", Sort: 2},
	{Name: "设置准入", Code: "user:setAdmission", Menu: "menu:user", Method: "PUT", Path: "/api/v1/users/*/admission", Sort: 3},
	{Name: "分配角色", Code: "user:setRoles", Menu: "menu:user", Method: "PUT", Path: "/api/v1/users/*/roles", Sort: 4},
	{Name: "移除用户", Code: "user:delete", Menu: "menu:user", Method: "DELETE", Path: "/api/v1/users/*", Sort: 5},
	{Name: "新增用户", Code: "user:add", Menu: "menu:user", Method: "POST", Path: "/api/v1/users", Sort: 6},
	{Name: "同步平台成员", Code: "user:sync", Menu: "menu:user", Method: "POST", Path: "/api/v1/users/sync", Sort: 7},
	{Name: "导出用户", Code: "user:export", Menu: "menu:user", Method: "POST", Path: "/api/v1/users/export", Sort: 8},
	// 角色管理
	{Name: "查询角色", Code: "role:list", Menu: "menu:role", Method: "GET", Path: "/api/v1/roles", Sort: 1},
	{Name: "角色详情", Code: "role:view", Menu: "menu:role", Method: "GET", Path: "/api/v1/roles/*", Sort: 2},
	{Name: "新增角色", Code: "role:create", Menu: "menu:role", Method: "POST", Path: "/api/v1/roles", Sort: 3},
	{Name: "编辑角色", Code: "role:update", Menu: "menu:role", Method: "PUT", Path: "/api/v1/roles/*", Sort: 4},
	{Name: "删除角色", Code: "role:delete", Menu: "menu:role", Method: "DELETE", Path: "/api/v1/roles/*", Sort: 5},
	{Name: "分配权限", Code: "role:setPermissions", Menu: "menu:role", Method: "PUT", Path: "/api/v1/roles/*/permissions", Sort: 6},
	// 菜单管理
	{Name: "查询权限", Code: "menu:list", Menu: "menu:menu", Method: "GET", Path: "/api/v1/permissions", Sort: 1},
	{Name: "权限详情", Code: "menu:view", Menu: "menu:menu", Method: "GET", Path: "/api/v1/permissions/*", Sort: 2},
	{Name: "新增权限", Code: "menu:create", Menu: "menu:menu", Method: "POST", Path: "/api/v1/permissions", Sort: 3},
	{Name: "编辑权限", Code: "menu:update", Menu: "menu:menu", Method: "PUT", Path: "/api/v1/permissions/*", Sort: 4},
	{Name: "删除权限", Code: "menu:delete", Menu: "menu:menu", Method: "DELETE", Path: "/api/v1/permissions/*", Sort: 5},
	// 设备管理（开放面代理）
	{Name: "查询设备", Code: "device:list", Menu: "menu:device", Method: "GET", Path: "/api/v1/devices", Sort: 1},
	{Name: "设备详情", Code: "device:view", Menu: "menu:device", Method: "GET", Path: "/api/v1/devices/*", Sort: 2},
	{Name: "注册设备", Code: "device:register", Menu: "menu:device", Method: "POST", Path: "/api/v1/devices", Sort: 3},
	{Name: "读取影子", Code: "device:shadow", Menu: "menu:device", Method: "GET", Path: "/api/v1/devices/*/shadow", Sort: 4},
	{Name: "查询命令", Code: "device:commandList", Menu: "menu:device", Method: "GET", Path: "/api/v1/devices/*/commands", Sort: 5},
	{Name: "查询遥测", Code: "device:telemetry", Menu: "menu:device", Method: "GET", Path: "/api/v1/devices/*/telemetry", Sort: 6},
	{Name: "查询事件", Code: "device:dataEvent", Menu: "menu:device", Method: "GET", Path: "/api/v1/devices/*/data-events", Sort: 7},
	{Name: "下发命令", Code: "device:commandIssue", Menu: "menu:device", Method: "POST", Path: "/api/v1/devices/*/commands", Sort: 8},
	{Name: "命令详情", Code: "device:commandGet", Menu: "menu:device", Method: "GET", Path: "/api/v1/device-commands/*", Sort: 9},
	// 型号管理（管理面代理）
	{Name: "查询型号", Code: "model:list", Menu: "menu:deviceModel", Method: "GET", Path: "/api/v1/device-models", Sort: 1},
	{Name: "型号详情", Code: "model:view", Menu: "menu:deviceModel", Method: "GET", Path: "/api/v1/device-models/*", Sort: 2},
	{Name: "新增型号", Code: "model:create", Menu: "menu:deviceModel", Method: "POST", Path: "/api/v1/device-models", Sort: 3},
	{Name: "编辑型号", Code: "model:update", Menu: "menu:deviceModel", Method: "PUT", Path: "/api/v1/device-models/*", Sort: 4},
	{Name: "删除型号", Code: "model:delete", Menu: "menu:deviceModel", Method: "DELETE", Path: "/api/v1/device-models/*", Sort: 5},
	{Name: "物模型选择器", Code: "model:tmPicker", Menu: "menu:deviceModel", Method: "GET", Path: "/api/v1/thing-models", Sort: 6},
	{Name: "物模型版本选择器", Code: "model:tmVersion", Menu: "menu:deviceModel", Method: "GET", Path: "/api/v1/thing-models/*/versions", Sort: 7},
	// 日志管理
	{Name: "查询操作日志", Code: "log:op:list", Menu: "menu:opLog", Method: "GET", Path: "/api/v1/operation-logs", Sort: 1},
	{Name: "清理操作日志", Code: "log:op:clear", Menu: "menu:opLog", Method: "DELETE", Path: "/api/v1/operation-logs", Sort: 2},
	{Name: "导出操作日志", Code: "log:op:export", Menu: "menu:opLog", Method: "POST", Path: "/api/v1/operation-logs/export", Sort: 3},
	// 文件管理
	{Name: "查询文件", Code: "file:list", Menu: "menu:file", Method: "GET", Path: "/api/v1/files", Sort: 1},
	{Name: "上传文件", Code: "file:upload", Menu: "menu:file", Method: "POST", Path: "/api/v1/files", Sort: 2},
	{Name: "下载文件", Code: "file:view", Menu: "menu:file", Method: "GET", Path: "/api/v1/files/*", Sort: 3},
	{Name: "删除文件", Code: "file:delete", Menu: "menu:file", Method: "DELETE", Path: "/api/v1/files/*", Sort: 4},
	// IP 黑名单
	{Name: "查询黑名单", Code: "blacklist:list", Menu: "menu:blacklist", Method: "GET", Path: "/api/v1/ip-blacklist", Sort: 1},
	{Name: "新增黑名单", Code: "blacklist:create", Menu: "menu:blacklist", Method: "POST", Path: "/api/v1/ip-blacklist", Sort: 2},
	{Name: "解封黑名单", Code: "blacklist:delete", Menu: "menu:blacklist", Method: "DELETE", Path: "/api/v1/ip-blacklist/*", Sort: 3},
	// 租户管理
	{Name: "查询租户", Code: "tenant:list", Menu: "menu:tenant", Method: "GET", Path: "/api/v1/tenants", Sort: 1},
	{Name: "租户详情", Code: "tenant:view", Menu: "menu:tenant", Method: "GET", Path: "/api/v1/tenants/*", Sort: 2},
	{Name: "新增租户", Code: "tenant:create", Menu: "menu:tenant", Method: "POST", Path: "/api/v1/tenants", Sort: 3},
	{Name: "编辑租户", Code: "tenant:update", Menu: "menu:tenant", Method: "PUT", Path: "/api/v1/tenants/*", Sort: 4},
	{Name: "删除租户", Code: "tenant:delete", Menu: "menu:tenant", Method: "DELETE", Path: "/api/v1/tenants/*", Sort: 5},
	{Name: "租户状态", Code: "tenant:status", Menu: "menu:tenant", Method: "PUT", Path: "/api/v1/tenants/*/status", Sort: 6},
	{Name: "同步至平台", Code: "tenant:sync", Menu: "menu:tenant", Method: "POST", Path: "/api/v1/tenants/*/sync", Sort: 7},
	// 应用用户
	{Name: "查询应用用户", Code: "appUser:list", Menu: "menu:appUser", Method: "GET", Path: "/api/v1/app-users", Sort: 1},
	{Name: "应用用户详情", Code: "appUser:view", Menu: "menu:appUser", Method: "GET", Path: "/api/v1/app-users/*", Sort: 2},
	{Name: "新增应用用户", Code: "appUser:create", Menu: "menu:appUser", Method: "POST", Path: "/api/v1/app-users", Sort: 3},
	{Name: "编辑应用用户", Code: "appUser:update", Menu: "menu:appUser", Method: "PUT", Path: "/api/v1/app-users/*", Sort: 4},
	{Name: "删除应用用户", Code: "appUser:delete", Menu: "menu:appUser", Method: "DELETE", Path: "/api/v1/app-users/*", Sort: 5},
	{Name: "重置密码", Code: "appUser:resetPwd", Menu: "menu:appUser", Method: "PUT", Path: "/api/v1/app-users/*/password", Sort: 6},
	// 服务器状态监控
	{Name: "查询服务器状态", Code: "monitor:list", Menu: "menu:monitor", Method: "GET", Path: "/api/v1/monitor", Sort: 1},
	// 智能体 —— 模型供应商（含供应商下的模型管理）
	{Name: "查询供应商", Code: "agent:provider:list", Menu: "menu:agentProvider", Method: "GET", Path: "/api/v1/agent/providers", Sort: 1},
	{Name: "供应商详情", Code: "agent:provider:view", Menu: "menu:agentProvider", Method: "GET", Path: "/api/v1/agent/providers/*", Sort: 2},
	{Name: "新增供应商", Code: "agent:provider:create", Menu: "menu:agentProvider", Method: "POST", Path: "/api/v1/agent/providers", Sort: 3},
	{Name: "编辑供应商", Code: "agent:provider:update", Menu: "menu:agentProvider", Method: "PUT", Path: "/api/v1/agent/providers/*", Sort: 4},
	{Name: "删除供应商", Code: "agent:provider:delete", Menu: "menu:agentProvider", Method: "DELETE", Path: "/api/v1/agent/providers/*", Sort: 5},
	{Name: "测试供应商", Code: "agent:provider:test", Menu: "menu:agentProvider", Method: "POST", Path: "/api/v1/agent/providers/*/test", Sort: 6},
	{Name: "拉取上游模型", Code: "agent:provider:remoteModels", Menu: "menu:agentProvider", Method: "GET", Path: "/api/v1/agent/providers/*/remote-models", Sort: 7},
	{Name: "查询模型", Code: "agent:model:list", Menu: "menu:agentProvider", Method: "GET", Path: "/api/v1/agent/models", Sort: 8},
	{Name: "新增模型", Code: "agent:model:create", Menu: "menu:agentProvider", Method: "POST", Path: "/api/v1/agent/models", Sort: 9},
	{Name: "编辑模型", Code: "agent:model:update", Menu: "menu:agentProvider", Method: "PUT", Path: "/api/v1/agent/models/*", Sort: 10},
	{Name: "删除模型", Code: "agent:model:delete", Menu: "menu:agentProvider", Method: "DELETE", Path: "/api/v1/agent/models/*", Sort: 11},
	{Name: "测试模型", Code: "agent:model:test", Menu: "menu:agentProvider", Method: "POST", Path: "/api/v1/agent/models/*/test", Sort: 12},
	// 智能体 —— Agent 配置
	{Name: "查询Agent", Code: "agent:list", Menu: "menu:agentList", Method: "GET", Path: "/api/v1/agents", Sort: 1},
	{Name: "Agent详情", Code: "agent:view", Menu: "menu:agentList", Method: "GET", Path: "/api/v1/agents/*", Sort: 2},
	{Name: "新增Agent", Code: "agent:create", Menu: "menu:agentList", Method: "POST", Path: "/api/v1/agents", Sort: 3},
	{Name: "编辑Agent", Code: "agent:update", Menu: "menu:agentList", Method: "PUT", Path: "/api/v1/agents/*", Sort: 4},
	{Name: "删除Agent", Code: "agent:delete", Menu: "menu:agentList", Method: "DELETE", Path: "/api/v1/agents/*", Sort: 5},
	{Name: "Agent调试对话", Code: "agent:chat", Menu: "menu:agentList", Method: "POST", Path: "/api/v1/agents/*/chat", Sort: 6},
	{Name: "查询工具清单", Code: "agent:tool:list", Menu: "menu:agentList", Method: "GET", Path: "/api/v1/agent/tools", Sort: 7},

	// 会话（本人数据，挂在聊天测试页）
	{Name: "查询会话", Code: "agent:conversation:list", Menu: "menu:agentChat", Method: "GET", Path: "/api/v1/agent/conversations", Sort: 1},
	{Name: "查询会话消息", Code: "agent:conversation:view", Menu: "menu:agentChat", Method: "GET", Path: "/api/v1/agent/conversations/*/messages", Sort: 2},
	{Name: "新建会话", Code: "agent:conversation:create", Menu: "menu:agentChat", Method: "POST", Path: "/api/v1/agent/conversations", Sort: 3},
	{Name: "重命名会话", Code: "agent:conversation:update", Menu: "menu:agentChat", Method: "PUT", Path: "/api/v1/agent/conversations/*", Sort: 4},
	{Name: "删除会话", Code: "agent:conversation:delete", Menu: "menu:agentChat", Method: "DELETE", Path: "/api/v1/agent/conversations/*", Sort: 5},

	// 用量统计
	{Name: "查询用量统计", Code: "agent:usage", Menu: "menu:agentUsage", Method: "GET", Path: "/api/v1/agent/usage", Sort: 1},

	// 数据字典
	{Name: "查询字典类型", Code: "dict:type:list", Menu: "menu:dict", Method: "GET", Path: "/api/v1/dict-types", Sort: 1},
	{Name: "字典类型详情", Code: "dict:type:view", Menu: "menu:dict", Method: "GET", Path: "/api/v1/dict-types/*", Sort: 2},
	{Name: "新增字典类型", Code: "dict:type:create", Menu: "menu:dict", Method: "POST", Path: "/api/v1/dict-types", Sort: 3},
	{Name: "编辑字典类型", Code: "dict:type:update", Menu: "menu:dict", Method: "PUT", Path: "/api/v1/dict-types/*", Sort: 4},
	{Name: "删除字典类型", Code: "dict:type:delete", Menu: "menu:dict", Method: "DELETE", Path: "/api/v1/dict-types/*", Sort: 5},
	{Name: "查询字典项", Code: "dict:item:list", Menu: "menu:dict", Method: "GET", Path: "/api/v1/dict-types/*/items", Sort: 6},
	{Name: "新增字典项", Code: "dict:item:create", Menu: "menu:dict", Method: "POST", Path: "/api/v1/dict-types/*/items", Sort: 7},
	{Name: "编辑字典项", Code: "dict:item:update", Menu: "menu:dict", Method: "PUT", Path: "/api/v1/dict-items/*", Sort: 8},
	{Name: "删除字典项", Code: "dict:item:delete", Menu: "menu:dict", Method: "DELETE", Path: "/api/v1/dict-items/*", Sort: 9},

	// 系统参数
	{Name: "查询参数", Code: "sysconfig:list", Menu: "menu:sysConfig", Method: "GET", Path: "/api/v1/sys-configs", Sort: 1},
	{Name: "新增参数", Code: "sysconfig:create", Menu: "menu:sysConfig", Method: "POST", Path: "/api/v1/sys-configs", Sort: 2},
	{Name: "编辑参数", Code: "sysconfig:update", Menu: "menu:sysConfig", Method: "PUT", Path: "/api/v1/sys-configs/*", Sort: 3},
	{Name: "删除参数", Code: "sysconfig:delete", Menu: "menu:sysConfig", Method: "DELETE", Path: "/api/v1/sys-configs/*", Sort: 4},

	// 通知公告（管理端发布；消费接口挂 basic 组，登录即可见）
	{Name: "查询公告", Code: "notice:list", Menu: "menu:notice", Method: "GET", Path: "/api/v1/notices", Sort: 1},
	{Name: "公告详情", Code: "notice:view", Menu: "menu:notice", Method: "GET", Path: "/api/v1/notices/*", Sort: 2},
	{Name: "发布公告", Code: "notice:create", Menu: "menu:notice", Method: "POST", Path: "/api/v1/notices", Sort: 3},
	{Name: "编辑公告", Code: "notice:update", Menu: "menu:notice", Method: "PUT", Path: "/api/v1/notices/*", Sort: 4},
	{Name: "删除公告", Code: "notice:delete", Menu: "menu:notice", Method: "DELETE", Path: "/api/v1/notices/*", Sort: 5},
}

// ensureSystemButtonPerms 幂等补齐系统管理接口权限点并绑定商户管理员角色（每次启动执行）：
//   - 按 code 查找，缺失则插入（自增 ID，避免与用户自建记录冲突）；
//   - 已存在（含软删残留）也同步校正为规范定义，自愈名称/接口归属变化（如菜单被删建后按钮成为孤儿节点）；
//   - 绑定商户管理员角色（ID=2，可用时）：内置角色的实际生效权限即来源于此。
func (d *Data) ensureSystemButtonPerms() error {
	// 菜单 code -> ID（存量库菜单 ID 可能与种子不同，按 code 解析；菜单缺失时 ParentID 落 0，不影响 RBAC）
	menuIDs := map[string]uint{}
	menuCodes := map[string]struct{}{}
	for _, def := range systemButtonPerms {
		menuCodes[def.Menu] = struct{}{}
	}
	for code := range menuCodes {
		var menu model.PermissionPO
		if err := d.DB.Where("code = ? AND type = ?", code, string(permission.TypeMenu)).First(&menu).Error; err == nil {
			menuIDs[code] = menu.ID
		}
	}

	permIDs := make([]uint, 0, len(systemButtonPerms))
	inserted := 0
	for _, def := range systemButtonPerms {
		var po model.PermissionPO
		err := d.DB.Unscoped().Where("code = ?", def.Code).First(&po).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// 缺失插入；软删残留恢复；已存在也校正归属与定义（自愈，如菜单删建后按钮孤儿化）
		if errors.Is(err, gorm.ErrRecordNotFound) {
			po = model.PermissionPO{Name: def.Name, Code: def.Code, Type: string(permission.TypeButton),
				Method: def.Method, Path: def.Path, ParentID: menuIDs[def.Menu], Sort: def.Sort}
			if err := d.DB.Create(&po).Error; err != nil {
				return err
			}
			inserted++
		} else {
			updates := map[string]interface{}{
				"name": def.Name, "type": string(permission.TypeButton),
				"method": def.Method, "path": def.Path, "parent_id": menuIDs[def.Menu], "sort": def.Sort,
			}
			if po.DeletedAt.Valid {
				updates["deleted_at"] = nil
				inserted++
			}
			if err := d.DB.Unscoped().Model(&po).Updates(updates).Error; err != nil {
				return err
			}
		}
		permIDs = append(permIDs, po.ID)
	}

	// 商户管理员角色随种子同步持有全部按钮权限点（可用时；通配权限不在种子清单内，
	// 角色2 永远不会拿到 all 通配）
	if merchantRoleBound {
		for _, pid := range permIDs {
			if err := d.bindRole(bizadmission.MerchantAdminRoleID, pid); err != nil {
				return err
			}
		}
	}

	if inserted > 0 {
		logger.Info("ensured system button permissions", zap.Int("inserted", inserted))
	}
	return nil
}

// obsoletePermCodes 已废弃的菜单与按钮权限点（本次改造移除的模块）：
// 在线用户/登录日志（会话与登录委外给平台）、开放API/商户/调用日志（商户模块移除）
var obsoletePermCodes = []string{
	"menu:online", "menu:loginLog", "menu:openapi", "menu:merchant", "menu:merchantLog",
	"online:list", "online:kick", "online:kickUser",
	"log:login:list", "log:login:clear", "log:login:export",
	"merchant:list", "merchant:view", "merchant:create", "merchant:update",
	"merchant:delete", "merchant:resetSecret", "merchant:status", "merchantLog:list",
	// 旧用户管理按钮（本地账号体系移除，由 user:setAdmission 等新按钮替代）
	"user:create", "user:update", "user:resetPassword",
}

// migrateLegacy 存量库幂等迁移（每次启动执行，无匹配行时零副作用）：
//  1. api 权限类型并入 button（接口绑定能力由 button 承担）；
//  2. 移除已并入「菜单管理」页的旧「权限管理」菜单入口（含角色关联）；
//  3. 「菜单与权限」更名「菜单管理」、「用户管理」更名「用户准入」；
//  4. 目录类型显式化：含菜单/目录子级的 menu 转为 dir（仅按钮子级的不算，避免页面菜单被误判为分组）；
//  5. 删除已废弃的 roles.code 列（AutoMigrate 不会删列，删列时唯一索引随之删除）；
//  6. 物理删除本次改造废弃的菜单/按钮权限点及其角色绑定（在线用户/登录日志/商户/开放API 等）；
//  7. 物理删除已废弃的内置超管角色（ID=1）及其用户/权限绑定与 all 通配权限行
//     （bootstrapAdmins 引导机制移除，商户管理员成为唯一内置角色）。
func (d *Data) migrateLegacy() error {
	if err := d.DB.Model(&model.PermissionPO{}).Where("type = ?", "api").
		Update("type", "button").Error; err != nil {
		return err
	}
	// MySQL 禁止在 UPDATE 子查询中直接引用目标表（Error 1093），包一层派生表绕过；PG/SQLite 同样兼容
	if err := d.DB.Exec(`UPDATE permissions SET type = 'dir' WHERE type = 'menu' AND id IN (SELECT parent_id FROM (SELECT DISTINCT parent_id FROM permissions WHERE parent_id <> 0 AND type IN ('menu', 'dir')) AS sub)`).Error; err != nil {
		return err
	}
	var legacyMenu model.PermissionPO
	err := d.DB.Where("code = ?", "menu:permission").First(&legacyMenu).Error
	switch {
	case err == nil:
		if err := d.DB.Delete(&model.RolePermissionPO{}, "permission_id = ?", legacyMenu.ID).Error; err != nil {
			return err
		}
		if err := d.DB.Delete(&legacyMenu).Error; err != nil {
			return err
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		// 不存在则跳过
	default:
		return err
	}
	if err := d.DB.Model(&model.PermissionPO{}).Where("code = ?", "menu:menu").
		Update("name", "菜单管理").Error; err != nil {
		return err
	}
	if err := d.DB.Model(&model.PermissionPO{}).Where("code = ?", "menu:user").
		Update("name", "用户准入").Error; err != nil {
		return err
	}
	// roles.code 已随编码功能移除：存量库显式删列（MySQL/PG/SQLite 删列均连带删除仅含该列的唯一索引）
	if d.DB.Migrator().HasColumn(&model.RolePO{}, "code") {
		if err := d.DB.Migrator().DropColumn(&model.RolePO{}, "code"); err != nil {
			return err
		}
		logger.Info("dropped legacy column roles.code")
	}
	// 废弃权限点清理：先删角色绑定，再物理删权限行（含软删残留）
	if len(obsoletePermCodes) > 0 {
		var ids []uint
		if err := d.DB.Unscoped().Model(&model.PermissionPO{}).
			Where("code IN ?", obsoletePermCodes).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			if err := d.DB.Unscoped().Delete(&model.RolePermissionPO{}, "permission_id IN ?", ids).Error; err != nil {
				return err
			}
			if err := d.DB.Unscoped().Delete(&model.PermissionPO{}, "id IN ?", ids).Error; err != nil {
				return err
			}
			logger.Info("dropped obsolete permissions (merchant/online/login-log/local-account)", zap.Int("count", len(ids)))
		}
	}

	// 内置超管角色（ID=1）随 bootstrapAdmins 机制移除：删除角色行、用户绑定、权限绑定与 all 通配权限。
	// 原引导账号的准入投影保留（可由商户管理员在用户准入页管理），本地角色绑定同步吊销。
	var allPermIDs []uint
	if err := d.DB.Unscoped().Model(&model.PermissionPO{}).
		Where("code = ?", "all").Pluck("id", &allPermIDs).Error; err != nil {
		return err
	}
	if err := d.DB.Unscoped().Where("role_id = ?", uint(1)).Delete(&model.PlatformUserRolePO{}).Error; err != nil {
		return err
	}
	if err := d.DB.Unscoped().Where("role_id = ?", uint(1)).Delete(&model.RolePermissionPO{}).Error; err != nil {
		return err
	}
	if len(allPermIDs) > 0 {
		if err := d.DB.Unscoped().Where("permission_id IN ?", allPermIDs).Delete(&model.RolePermissionPO{}).Error; err != nil {
			return err
		}
		if err := d.DB.Unscoped().Where("id IN ?", allPermIDs).Delete(&model.PermissionPO{}).Error; err != nil {
			return err
		}
	}
	res := d.DB.Unscoped().Where("id = ?", uint(1)).Delete(&model.RolePO{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		logger.Info("dropped legacy super admin role (bootstrapAdmins removed; merchant admin is the only built-in role)")
	}
	return nil
}
