// Package conf 配置加载（viper），对应 Kratos 的 internal/conf。
package conf

import (
	"strings"

	"github.com/spf13/viper"
)

// Bootstrap 应用根配置
type Bootstrap struct {
	Server   Server   `mapstructure:"server"`
	DB       DB       `mapstructure:"db"`
	JWT      JWT      `mapstructure:"jwt"`
	Redis    Redis    `mapstructure:"redis"`
	Auth     Auth     `mapstructure:"auth"`
	Log      Log      `mapstructure:"log"`
	Cache    Cache    `mapstructure:"cache"`
	Storage  Storage  `mapstructure:"storage"`
	Export   Export   `mapstructure:"export"`
	Platform Platform `mapstructure:"platform"`
	Notify   Notify   `mapstructure:"notify"`
	Agent    Agent    `mapstructure:"agent"`
}

type Server struct {
	Port      int    `mapstructure:"port"`
	Mode      string `mapstructure:"mode"`
	StaticDir string `mapstructure:"staticDir"` // 前端产物目录（如 web/dist），存在则由后端托管 SPA
	// CORSOrigins 跨域来源白名单：空 = 同源模式（不下发任何 CORS 头，前后端同源托管
	// 与本地 vite 代理均无需配置）；配置后仅命中来源回显并允许凭证；
	// 显式配置 ["*"] 恢复通配（历史行为，不允许凭证）。环境变量 APP_SERVER_CORSORIGINS（逗号分隔）
	CORSOrigins []string `mapstructure:"corsOrigins"`
	// TrustedProxies 可信代理列表（IP 或 CIDR）：仅这些代理设置的 X-Forwarded-For 参与
	// ClientIP 解析，其余一律取直连地址。必须配置——否则登录限流/封禁/审计记录的 IP
	// 可被请求头伪造绕过。直连部署留空；反向代理（nginx 等）后配置代理机地址，
	// 如 ["127.0.0.1"]。环境变量 APP_SERVER_TRUSTEDPROXIES（逗号分隔）
	TrustedProxies []string `mapstructure:"trustedProxies"`
}

type DB struct {
	Driver      string   `mapstructure:"driver"`
	AutoMigrate bool     `mapstructure:"autoMigrate"`
	MySQL       MySQL    `mapstructure:"mysql"`
	Postgres    Postgres `mapstructure:"postgres"`
	SQLite      SQLite   `mapstructure:"sqlite"`
}

type MySQL struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	Charset      string `mapstructure:"charset"`
	MaxOpenConns int    `mapstructure:"maxOpenConns"`
	MaxIdleConns int    `mapstructure:"maxIdleConns"`
}

type Postgres struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"maxOpenConns"`
	MaxIdleConns int    `mapstructure:"maxIdleConns"`
}

type SQLite struct {
	Path string `mapstructure:"path"`
}

type JWT struct {
	Secret       string `mapstructure:"secret"`
	Issuer       string `mapstructure:"issuer"`
	ExpireHours  int    `mapstructure:"expireHours"`
	RefreshHours int    `mapstructure:"refreshHours"`
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type Auth struct {
	// CaptchaEnabled 登录图形验证码开关（本地调试可临时关闭，生产必须开启）
	CaptchaEnabled bool `mapstructure:"captchaEnabled"`
}

// Cache 二级缓存配置（L1 进程内存 + L2 Redis）
type Cache struct {
	// L2Enabled 二级缓存（Redis）开关；false 时 RBAC 等读缓存仅用进程内 L1，多实例间不共享
	L2Enabled bool `mapstructure:"l2Enabled"`
}

// Platform embodied-platform 平台接入配置。
// 平台是唯一身份源（登录在本系统前端直调平台，本系统后端只做 token 自省 + 本地准入投影）；
// 设备/型号走平台开放面（商户 HMAC）与管理面（服务账号 JWT）；文件存储走 storage-gateway。
type Platform struct {
	// BaseURL 平台主服务地址（/api/v1 与 /open-api/v1 同源），形如 http://host:27080
	BaseURL string `mapstructure:"baseUrl"`
	// AppKey / AppSecret 商户凭证（平台商户管理创建，AppSecret 明文仅创建时返回一次）
	AppKey    string `mapstructure:"appKey"`
	AppSecret string `mapstructure:"appSecret"`
	// Storage storage-gateway 直连（文件存储后端；API Key 由平台管理端 /api/v1/storage/api-keys 签发）
	Storage PlatformStorage `mapstructure:"storage"`
	// TimeoutSeconds 调用平台的 HTTP 超时（秒）
	TimeoutSeconds int `mapstructure:"timeoutSeconds"`
}

// PlatformStorage storage-gateway OpenAPI 接入（Bearer {apiKeyId}:{apiSecret}）
type PlatformStorage struct {
	// BaseURL 网关地址（前缀 /openapi/storage/v1 由客户端拼接），形如 http://host:27091
	BaseURL   string `mapstructure:"baseUrl"`
	APIKeyID  string `mapstructure:"apiKeyId"`
	APISecret string `mapstructure:"apiSecret"`
	// Bucket 文件管理统一使用的桶（需在 API Key scope 内）
	Bucket string `mapstructure:"bucket"`
}

// Enabled 平台接入是否已配置（BaseURL 非空视为启用；未配置时同步类操作报错）
func (p Platform) Enabled() bool { return p.BaseURL != "" }

// Agent 智能体模块配置（LLM 配置底座）
type Agent struct {
	// CryptoKey 供应商 API Key 的 AES 加密密钥（任意长度，内部派生 AES-256）；
	// 为空时从 jwt.secret 派生 —— 更换 jwt.secret 会使已存密钥不可解密（错误表现为 agent.decrypt_failed），
	// 重新保存一次供应商密钥即可恢复
	CryptoKey string `mapstructure:"cryptoKey"`
	// UsageRetentionDays 用量计量流水保留天数，超期每日自动清理；0 表示永久保留
	UsageRetentionDays int `mapstructure:"usageRetentionDays"`
}

// Notify 告警通知模块配置（渠道密钥加密；告警规则/渠道均存库、运行时管理）
type Notify struct {
	// CryptoKey 通知渠道密钥（SMTP 密码/Webhook 密钥）的 AES 加密密钥（任意长度，内部派生 AES-256）；
	// 为空时从 jwt.secret 派生，域前缀与 agent 隔离 —— 更换 jwt.secret 后需重新保存一次渠道密钥
	CryptoKey string `mapstructure:"cryptoKey"`
}
type Log struct {
	// RetentionDays 登录/操作日志保留天数，超期每日自动清理；0 表示永久保留
	RetentionDays int `mapstructure:"retentionDays"`
	// Dir 应用运行日志文件目录
	Dir string `mapstructure:"dir"`
	// Filename 日志文件名前缀（实际文件为 prefix-2006-01-02.log，按日期滚动）
	Filename string `mapstructure:"filename"`
	// Level 最低日志级别（debug|info|warn|error）；空则 debug 环境=debug，release=info
	Level string `mapstructure:"level"`
	// MaxAgeDays 日志文件保留天数，超期每日自动清理；0 表示永久保留
	MaxAgeDays int `mapstructure:"maxAgeDays"`
	// Console 是否同时输出到控制台（debug 默认开，release 默认关）
	Console bool `mapstructure:"console"`
}

// Export 异步导出配置：产物经存储后端（storage.driver）落盘，到期自动清理
type Export struct {
	MaxSizeMB     int64             `mapstructure:"maxSizeMB"`     // 单文件大小上限（MB），超出截断
	MaxRows       int               `mapstructure:"maxRows"`       // 单文件最大数据行数，超出截断
	RetentionDays int               `mapstructure:"retentionDays"` // 导出记录保留天数，超期每日自动清理；0 表示永久保留
	QueueSize     int               `mapstructure:"queueSize"`     // 导出任务队列容量，满则拒绝新任务
	Mask          map[string]string `mapstructure:"mask"`          // 导出字段名 -> 脱敏规则（phone/email/idcard/bankcard/name/ip/none）
}

// Storage 文件存储配置：driver 决定新上传写入的后端；
// 读/删按文件记录落库时的 driver 解析对应后端，因此切换 driver 后旧文件仍可访问
type Storage struct {
	Driver            string       `mapstructure:"driver"` // platform | local（平台存储未配置时自动降级 local）
	MaxSizeMB         int64        `mapstructure:"maxSizeMB"`
	SignExpireMinutes int          `mapstructure:"signExpireMinutes"` // 预签名下载 URL 有效期（分钟）
	DenyExts          []string     `mapstructure:"denyExts"`          // 禁止上传的扩展名（空则用内置黑名单）
	Local             LocalStorage `mapstructure:"local"`
}

type LocalStorage struct {
	Dir string `mapstructure:"dir"` // 本地存储根目录（降级写入后端 + 历史存量文件读取）
}

// Load 从指定路径加载配置
func Load(path string) (*Bootstrap, error) {
	v := viper.New()
	v.SetConfigFile(path)
	// 环境变量覆盖：APP_ 前缀 + 配置路径下划线拼接（. → _），
	// 如 APP_DB_MYSQL_HOST 覆盖 db.mysql.host、APP_JWT_SECRET 覆盖 jwt.secret，
	// 优先级高于配置文件，供容器部署注入连接信息与密钥
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	// 默认值：验证码默认开启，未配置时行为不变
	v.SetDefault("auth.captchaEnabled", true)
	// 默认值：二级缓存（Redis）默认开启
	v.SetDefault("cache.l2Enabled", true)
	// 默认值：日志默认保留 90 天
	v.SetDefault("log.retentionDays", 90)
	// 默认值：运行日志按日滚动落盘 ./logs，保留 30 天
	v.SetDefault("log.dir", "./logs")
	v.SetDefault("log.filename", "app")
	v.SetDefault("log.maxAgeDays", 30)
	// 默认值：异步导出单文件 50MB / 10 万行，产物保留 7 天，任务队列 64
	v.SetDefault("export.maxSizeMB", 50)
	v.SetDefault("export.maxRows", 100000)
	v.SetDefault("export.retentionDays", 7)
	v.SetDefault("export.queueSize", 64)
	// 默认值：文件上传上限 20MB，预签名 URL 15 分钟，本地降级目录 ./data/uploads
	v.SetDefault("storage.maxSizeMB", 20)
	v.SetDefault("storage.signExpireMinutes", 15)
	v.SetDefault("storage.local.dir", "./data/uploads")
	// 默认值：平台调用超时 15s；文件桶缺省 default
	v.SetDefault("platform.timeoutSeconds", 15)
	v.SetDefault("platform.storage.bucket", "default")
	// 默认值：agent 用量流水保留 90 天
	v.SetDefault("agent.usageRetentionDays", 90)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var c Bootstrap
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

// LoadDefault 从默认路径列表加载（项目根 configs/config.yaml 或 -conf 指定路径）
func LoadDefault() (*Bootstrap, error) {
	paths := []string{"configs/config.yaml", "conf/config.yaml", "config.yaml"}
	for _, p := range paths {
		if c, err := Load(p); err == nil {
			return c, nil
		}
	}
	return nil, viper.ConfigFileNotFoundError{}
}
