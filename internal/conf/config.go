// Package conf 配置加载（viper），对应 Kratos 的 internal/conf。
package conf

import (
	"github.com/spf13/viper"
)

// Bootstrap 应用根配置
type Bootstrap struct {
	Server Server `mapstructure:"server"`
	DB     DB     `mapstructure:"db"`
	JWT    JWT    `mapstructure:"jwt"`
	Redis  Redis  `mapstructure:"redis"`
	Auth    Auth    `mapstructure:"auth"`
	Log     Log     `mapstructure:"log"`
	Cache   Cache   `mapstructure:"cache"`
	Storage Storage `mapstructure:"storage"`
	Export  Export  `mapstructure:"export"`
	OpenAPI OpenAPI `mapstructure:"openapi"`
}

type Server struct {
	Port      int    `mapstructure:"port"`
	Mode      string `mapstructure:"mode"`
	StaticDir string `mapstructure:"staticDir"` // 前端产物目录（如 web/dist），存在则由后端托管 SPA
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

// OpenAPI 开放 API 验签配置（商户 HMAC 签名）
type OpenAPI struct {
	// SignSkewSeconds 请求时间戳允许的最大偏差（秒），超出即拒绝（防重放窗口）
	SignSkewSeconds int `mapstructure:"signSkewSeconds"`
	// NonceTTLSeconds nonce 防重放去重的 Redis 键 TTL（秒），应不小于 SignSkewSeconds
	NonceTTLSeconds int `mapstructure:"nonceTtlSeconds"`
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
// 读/删按文件记录落库时的 driver 解析对应后端，因此切换 driver 后旧文件仍可访问（旧后端配置需保留）
type Storage struct {
	Driver            string   `mapstructure:"driver"` // local | oss | cos | tos | minio
	MaxSizeMB         int64    `mapstructure:"maxSizeMB"`
	SignExpireMinutes int      `mapstructure:"signExpireMinutes"` // 云存储预签名下载 URL 有效期
	DenyExts          []string `mapstructure:"denyExts"`          // 禁止上传的扩展名（空则用内置黑名单）
	Local             LocalStorage `mapstructure:"local"`
	OSS               OSSStorage   `mapstructure:"oss"`
	COS               COSStorage   `mapstructure:"cos"`
	TOS               TOSStorage   `mapstructure:"tos"`
	MinIO             MinIOStorage `mapstructure:"minio"`
}

type LocalStorage struct {
	Dir string `mapstructure:"dir"` // 本地存储根目录
}

// OSSStorage 阿里云 OSS
type OSSStorage struct {
	Endpoint        string `mapstructure:"endpoint"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"accessKeyId"`
	AccessKeySecret string `mapstructure:"accessKeySecret"`
	Prefix          string `mapstructure:"prefix"` // 对象 key 前缀（如 uploads/）
}

// COSStorage 腾讯云 COS
type COSStorage struct {
	Region    string `mapstructure:"region"`
	Bucket    string `mapstructure:"bucket"` // 形如 example-1250000000
	SecretID  string `mapstructure:"secretId"`
	SecretKey string `mapstructure:"secretKey"`
	Prefix    string `mapstructure:"prefix"`
}

// TOSStorage 火山引擎 TOS
type TOSStorage struct {
	Endpoint  string `mapstructure:"endpoint"`
	Region    string `mapstructure:"region"`
	Bucket    string `mapstructure:"bucket"`
	AccessKey string `mapstructure:"accessKey"`
	SecretKey string `mapstructure:"secretKey"`
	Prefix    string `mapstructure:"prefix"`
}

// MinIOStorage 自定义 S3 兼容存储（如 JuiceFS + MinIO）
type MinIOStorage struct {
	Endpoint  string `mapstructure:"endpoint"` // 不含 scheme，如 127.0.0.1:9000
	Bucket    string `mapstructure:"bucket"`
	AccessKey string `mapstructure:"accessKey"`
	SecretKey string `mapstructure:"secretKey"`
	UseSSL    bool   `mapstructure:"useSSL"`
	Prefix    string `mapstructure:"prefix"`
}

// Load 从指定路径加载配置
func Load(path string) (*Bootstrap, error) {
	v := viper.New()
	v.SetConfigFile(path)
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
	// 默认值：文件存储默认本地驱动，上传上限 20MB，预签名 URL 15 分钟
	v.SetDefault("storage.driver", "local")
	v.SetDefault("storage.maxSizeMB", 20)
	v.SetDefault("storage.signExpireMinutes", 15)
	v.SetDefault("storage.local.dir", "./data/uploads")
	// 默认值：开放 API 签名时间戳允许偏差 300 秒，nonce 防重放 TTL 600 秒
	v.SetDefault("openapi.signSkewSeconds", 300)
	v.SetDefault("openapi.nonceTtlSeconds", 600)
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
