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
	"github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/biz/user"
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

// migrateAndSeed 自动建表 + 存量迁移 + 种子数据（超管 admin/123456）
func (d *Data) migrateAndSeed() error {
	if err := d.DB.AutoMigrate(
		&model.UserPO{}, &model.RolePO{}, &model.PermissionPO{},
		&model.UserRolePO{}, &model.RolePermissionPO{},
		&model.LoginLogPO{}, &model.OperationLogPO{},
		&model.FilePO{}, &model.ExportRecordPO{}, &model.IPBlacklistPO{},
		&model.MerchantPO{}, &model.MerchantAPILogPO{},
		&model.TenantPO{}, &model.AppUserPO{}, &model.AppUserTenantPO{},
	); err != nil {
		return err
	}

	if err := d.migrateLegacy(); err != nil {
		return err
	}

	var userCount int64
	if err := d.DB.Model(&model.UserPO{}).Count(&userCount).Error; err != nil {
		return err
	}
	if userCount == 0 {
		// 超管角色 + 通配权限（button 绑定 */*，参与 RBAC 匹配）
		rolePO := model.RolePO{ID: 1, Name: "超级管理员", Remark: "拥有全部权限"}
		permPO := model.PermissionPO{ID: 1, Name: "全部权限", Code: "all", Type: string(permission.TypeButton), Method: "*", Path: "*"}

		// 菜单种子数据（接口按钮权限点由 ensureSystemButtonPerms 统一补齐）
		perms := []model.PermissionPO{
			{ID: 100, Name: "首页", Code: "menu:dashboard", Type: "menu", Path: "/dashboard", Icon: "HomeOutline", Sort: 1},
			{ID: 110, Name: "系统管理", Code: "menu:system", Type: "dir", Icon: "SettingsOutline", Sort: 2},
			{ID: 111, Name: "用户管理", Code: "menu:user", Type: "menu", Path: "/system/users", ParentID: 110, Icon: "PersonOutline", Sort: 1},
			{ID: 112, Name: "角色管理", Code: "menu:role", Type: "menu", Path: "/system/roles", ParentID: 110, Icon: "IdCardOutline", Sort: 2},
			{ID: 114, Name: "菜单管理", Code: "menu:menu", Type: "menu", Path: "/system/menus", ParentID: 110, Icon: "MenuOutline", Sort: 4},
			{ID: 115, Name: "在线用户", Code: "menu:online", Type: "menu", Path: "/system/online", ParentID: 110, Icon: "PulseOutline", Sort: 5},
		}
		adminPwd, err := user.NewPassword("123456")
		if err != nil {
			return err
		}
		adminPO := model.UserPO{ID: 1, Username: "admin", Password: string(adminPwd), Nickname: "超级管理员", Email: "admin@smilex.local", Status: int(user.StatusEnabled)}

		if err := d.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&permPO).Error; err != nil {
				return err
			}
			if err := tx.Create(&rolePO).Error; err != nil {
				return err
			}
			if err := tx.Create(&adminPO).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.UserRolePO{UserID: 1, RoleID: 1}).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.RolePermissionPO{RoleID: 1, PermissionID: 1}).Error; err != nil {
				return err
			}
			for i := range perms {
				if err := tx.Create(&perms[i]).Error; err != nil {
					return err
				}
				if err := tx.Create(&model.RolePermissionPO{RoleID: 1, PermissionID: perms[i].ID}).Error; err != nil {
					return err
				}
			}
			logger.Info("seeded super admin: admin/123456 (please change password before production)")
			return nil
		}); err != nil {
			return err
		}
	}

	// 系统菜单幂等补齐（存量库升级路径），需先于按钮补齐执行以解析按钮归属
	if err := d.ensureSystemMenus(); err != nil {
		return err
	}
	// 系统管理接口权限点补齐并绑定超管角色（存量库/全新库统一走此路径，幂等）
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
	{Name: "在线用户", Code: "menu:online", Path: "/system/online", Icon: "PulseOutline", Sort: 5, ParentCode: "menu:system"},
	{Name: "关于我们", Code: "menu:about", Path: "/about", Icon: "InformationCircleOutline", Sort: 9},
	// 日志管理（顶级目录分组，父级先于子菜单声明以解析 ParentCode）
	{Name: "日志管理", Code: "menu:log", Type: "dir", Icon: "DocumentTextOutline", Sort: 3},
	{Name: "登录日志", Code: "menu:loginLog", Path: "/log/login-logs", Icon: "LogInOutline", Sort: 1, ParentCode: "menu:log"},
	{Name: "操作日志", Code: "menu:opLog", Path: "/log/operation-logs", Icon: "ClipboardOutline", Sort: 2, ParentCode: "menu:log"},
	// 文件管理（顶级菜单）
	{Name: "文件管理", Code: "menu:file", Path: "/file", Icon: "FolderOpenOutline", Sort: 4},
	// IP 黑名单（挂在系统管理目录下）
	{Name: "IP黑名单", Code: "menu:blacklist", Path: "/system/blacklist", Icon: "BanOutline", Sort: 6, ParentCode: "menu:system"},
	// 开放API（顶级目录分组，父级先于子菜单声明以解析 ParentCode）
	{Name: "开放API", Code: "menu:openapi", Type: "dir", Icon: "KeyOutline", Sort: 5},
	{Name: "商户管理", Code: "menu:merchant", Path: "/openapi/merchants", Icon: "StorefrontOutline", Sort: 1, ParentCode: "menu:openapi"},
	{Name: "API调用日志", Code: "menu:merchantLog", Path: "/openapi/api-logs", Icon: "DocumentTextOutline", Sort: 2, ParentCode: "menu:openapi"},
	// 租户中心（顶级目录分组，父级先于子菜单声明以解析 ParentCode）
	{Name: "租户中心", Code: "menu:tenantCenter", Type: "dir", Icon: "BusinessOutline", Sort: 2},
	{Name: "租户管理", Code: "menu:tenant", Path: "/tenant/tenants", Icon: "BusinessOutline", Sort: 1, ParentCode: "menu:tenantCenter"},
	{Name: "应用用户", Code: "menu:appUser", Path: "/tenant/app-users", Icon: "PeopleOutline", Sort: 2, ParentCode: "menu:tenantCenter"},
}

// ensureSystemMenus 幂等补齐系统菜单并绑定超管角色（每次启动执行）：
// 按 code 查找（type 兼容 menu/dir，存量迁移会翻转类型），缺失则插入（父级按 code 解析，缺失时落为顶级菜单），并绑定超管角色 ID=1
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
		if err := d.bindSuperRole(po.ID); err != nil {
			return err
		}
	}
	return nil
}

// bindSuperRole 将权限绑定到超管角色（ID=1，幂等；超管已有 * 通配，绑定仅为角色权限树回显一致）
func (d *Data) bindSuperRole(permID uint) error {
	var cnt int64
	if err := d.DB.Model(&model.RolePermissionPO{}).
		Where("role_id = ? AND permission_id = ?", 1, permID).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	return d.DB.Create(&model.RolePermissionPO{RoleID: 1, PermissionID: permID}).Error
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
	// 用户管理
	{Name: "查询用户", Code: "user:list", Menu: "menu:user", Method: "GET", Path: "/api/v1/users", Sort: 1},
	{Name: "用户详情", Code: "user:view", Menu: "menu:user", Method: "GET", Path: "/api/v1/users/*", Sort: 2},
	{Name: "新增用户", Code: "user:create", Menu: "menu:user", Method: "POST", Path: "/api/v1/users", Sort: 3},
	{Name: "编辑用户", Code: "user:update", Menu: "menu:user", Method: "PUT", Path: "/api/v1/users/*", Sort: 4},
	{Name: "删除用户", Code: "user:delete", Menu: "menu:user", Method: "DELETE", Path: "/api/v1/users/*", Sort: 5},
	{Name: "分配角色", Code: "user:setRoles", Menu: "menu:user", Method: "PUT", Path: "/api/v1/users/*/roles", Sort: 6},
	{Name: "重置密码", Code: "user:resetPassword", Menu: "menu:user", Method: "PUT", Path: "/api/v1/users/*/password", Sort: 7},
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
	// 在线用户
	{Name: "查询在线用户", Code: "online:list", Menu: "menu:online", Method: "GET", Path: "/api/v1/online-users", Sort: 1},
	{Name: "下线会话", Code: "online:kick", Menu: "menu:online", Method: "DELETE", Path: "/api/v1/online-users/*", Sort: 2},
	{Name: "用户全部下线", Code: "online:kickUser", Menu: "menu:online", Method: "DELETE", Path: "/api/v1/users/*/sessions", Sort: 3},
	// 日志管理
	{Name: "查询登录日志", Code: "log:login:list", Menu: "menu:loginLog", Method: "GET", Path: "/api/v1/login-logs", Sort: 1},
	{Name: "清理登录日志", Code: "log:login:clear", Menu: "menu:loginLog", Method: "DELETE", Path: "/api/v1/login-logs", Sort: 2},
	{Name: "导出登录日志", Code: "log:login:export", Menu: "menu:loginLog", Method: "POST", Path: "/api/v1/login-logs/export", Sort: 3},
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
	// 商户管理（开放 API 授权）
	{Name: "查询商户", Code: "merchant:list", Menu: "menu:merchant", Method: "GET", Path: "/api/v1/merchants", Sort: 1},
	{Name: "商户详情", Code: "merchant:view", Menu: "menu:merchant", Method: "GET", Path: "/api/v1/merchants/*", Sort: 2},
	{Name: "新增商户", Code: "merchant:create", Menu: "menu:merchant", Method: "POST", Path: "/api/v1/merchants", Sort: 3},
	{Name: "编辑商户", Code: "merchant:update", Menu: "menu:merchant", Method: "PUT", Path: "/api/v1/merchants/*", Sort: 4},
	{Name: "删除商户", Code: "merchant:delete", Menu: "menu:merchant", Method: "DELETE", Path: "/api/v1/merchants/*", Sort: 5},
	{Name: "重置密钥", Code: "merchant:resetSecret", Menu: "menu:merchant", Method: "PUT", Path: "/api/v1/merchants/*/secret", Sort: 6},
	{Name: "修改状态", Code: "merchant:status", Menu: "menu:merchant", Method: "PUT", Path: "/api/v1/merchants/*/status", Sort: 7},
	// API 调用日志
	{Name: "查询调用日志", Code: "merchantLog:list", Menu: "menu:merchantLog", Method: "GET", Path: "/api/v1/merchant-api-logs", Sort: 1},
	// 租户管理
	{Name: "查询租户", Code: "tenant:list", Menu: "menu:tenant", Method: "GET", Path: "/api/v1/tenants", Sort: 1},
	{Name: "租户详情", Code: "tenant:view", Menu: "menu:tenant", Method: "GET", Path: "/api/v1/tenants/*", Sort: 2},
	{Name: "新增租户", Code: "tenant:create", Menu: "menu:tenant", Method: "POST", Path: "/api/v1/tenants", Sort: 3},
	{Name: "编辑租户", Code: "tenant:update", Menu: "menu:tenant", Method: "PUT", Path: "/api/v1/tenants/*", Sort: 4},
	{Name: "删除租户", Code: "tenant:delete", Menu: "menu:tenant", Method: "DELETE", Path: "/api/v1/tenants/*", Sort: 5},
	// 应用用户
	{Name: "查询应用用户", Code: "appUser:list", Menu: "menu:appUser", Method: "GET", Path: "/api/v1/app-users", Sort: 1},
	{Name: "应用用户详情", Code: "appUser:view", Menu: "menu:appUser", Method: "GET", Path: "/api/v1/app-users/*", Sort: 2},
	{Name: "新增应用用户", Code: "appUser:create", Menu: "menu:appUser", Method: "POST", Path: "/api/v1/app-users", Sort: 3},
	{Name: "编辑应用用户", Code: "appUser:update", Menu: "menu:appUser", Method: "PUT", Path: "/api/v1/app-users/*", Sort: 4},
	{Name: "删除应用用户", Code: "appUser:delete", Menu: "menu:appUser", Method: "DELETE", Path: "/api/v1/app-users/*", Sort: 5},
	{Name: "重置密码", Code: "appUser:resetPwd", Menu: "menu:appUser", Method: "PUT", Path: "/api/v1/app-users/*/password", Sort: 6},
}

// ensureSystemButtonPerms 幂等补齐系统管理接口权限点并绑定超管角色（每次启动执行）：
//   - 按 code 查找，缺失则插入（自增 ID，避免与用户自建记录冲突）；
//   - 已存在（含软删残留）也同步校正为规范定义，自愈名称/接口归属变化（如菜单被删建后按钮成为孤儿节点）；
//   - 绑定超管角色（ID=1）：超管已有 * 通配实际全通过，绑定仅为角色权限树回显一致。
func (d *Data) ensureSystemButtonPerms() error {
	// 菜单 code -> ID（存量库菜单 ID 可能与种子不同，按 code 解析；菜单缺失时 ParentID 落 0，不影响 RBAC）
	menuIDs := map[string]uint{}
	for _, code := range []string{"menu:user", "menu:role", "menu:menu", "menu:online", "menu:loginLog", "menu:opLog", "menu:file", "menu:blacklist", "menu:merchant", "menu:merchantLog", "menu:tenant", "menu:appUser"} {
		var menu model.PermissionPO
		if err := d.DB.Where("code = ? AND type = ?", code, string(permission.TypeMenu)).First(&menu).Error; err == nil {
			menuIDs[code] = menu.ID
		}
	}

	// 超管角色固定 ID=1（编码字段已移除，角色不再有业务编码）
	var superRole model.RolePO
	superErr := d.DB.First(&superRole, 1).Error

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

	bound := 0
	if superErr == nil {
		for _, pid := range permIDs {
			var cnt int64
			if err := d.DB.Model(&model.RolePermissionPO{}).
				Where("role_id = ? AND permission_id = ?", superRole.ID, pid).Count(&cnt).Error; err != nil {
				return err
			}
			if cnt > 0 {
				continue
			}
			if err := d.DB.Create(&model.RolePermissionPO{RoleID: superRole.ID, PermissionID: pid}).Error; err != nil {
				return err
			}
			bound++
		}
	} else if !errors.Is(superErr, gorm.ErrRecordNotFound) {
		return superErr
	}

	if inserted > 0 || bound > 0 {
		logger.Info("ensured system button permissions",
			zap.Int("inserted", inserted), zap.Int("bound_to_super_admin", bound))
	}
	return nil
}

// migrateLegacy 存量库幂等迁移（每次启动执行，无匹配行时零副作用）：
//  1. api 权限类型并入 button（接口绑定能力由 button 承担）；
//  2. 移除已并入「菜单管理」页的旧「权限管理」菜单入口（含角色关联）；
//  3. 「菜单与权限」更名「菜单管理」；
//  4. 目录类型显式化：含菜单/目录子级的 menu 转为 dir（仅按钮子级的不算，避免页面菜单被误判为分组）；
//  5. 删除已废弃的 roles.code 列（AutoMigrate 不会删列，删列时唯一索引随之删除）
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
	// roles.code 已随编码功能移除：存量库显式删列（MySQL/PG/SQLite 删列均连带删除仅含该列的唯一索引）
	if d.DB.Migrator().HasColumn(&model.RolePO{}, "code") {
		if err := d.DB.Migrator().DropColumn(&model.RolePO{}, "code"); err != nil {
			return err
		}
		logger.Info("dropped legacy column roles.code")
	}
	return nil
}
