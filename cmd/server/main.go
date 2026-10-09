// SmileX-Admin-Gin 服务入口
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"go.uber.org/zap"
)

var confPath = flag.String("conf", "configs/config.yaml", "config file path")

// ProvideConfig wire Provider：加载配置
func ProvideConfig() (*conf.Bootstrap, error) {
	return conf.Load(*confPath)
}

func main() {
	flag.Parse()

	cfg, err := ProvideConfig()
	if err != nil {
		// 回退到默认路径
		cfg, err = conf.LoadDefault()
		if err != nil {
			panic("load config: " + err.Error())
		}
	}
	if err := logger.Init(cfg.Server.Mode, logger.Config{
		Dir:        cfg.Log.Dir,
		Filename:   cfg.Log.Filename,
		Level:      cfg.Log.Level,
		MaxAgeDays: cfg.Log.MaxAgeDays,
		Console:    cfg.Log.Console,
	}); err != nil {
		panic(err)
	}
	defer logger.Sync()

	app, cleanup, err := wireApp()
	if err != nil {
		panic("wire: " + err.Error())
	}
	defer cleanup()

	// 平台连通性自检（异步、不阻断启动：失败仅告警，具体同步/代理操作会报明确错误）
	if cfg.Platform.Enabled() {
		go checkPlatform(cfg)
		// 权限码注册表对账（目录动态化契约；依赖商户 HMAC，scope 前置 tenant-role:syncPerms）
		if cfg.Platform.AppKey != "" {
			go syncTenantPermCatalog(cfg)
		}
	}

	go func() {
		logger.Info("http server listening", zap.Int("port", cfg.Server.Port))
		if err := app.Start(); err != nil {
			logger.Error("server exit", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Stop(ctx); err != nil {
		logger.Error("server shutdown", zap.Error(err))
	}
	logger.Info("server stopped")
}

// checkPlatform 凭证连通性自检：开放面商户 HMAC / storage-gateway
func checkPlatform(cfg *conf.Bootstrap) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if cfg.Platform.AppKey != "" {
		if _, err := platform.NewOpenAPIClient(cfg).Ping(ctx); err != nil {
			logger.Warn("platform open-api（商户 HMAC）验签失败或不可达（设备/租户同步/型号管理将失败）", zap.Error(err))
		} else {
			logger.Info("platform open-api ok（设备/租户同步/型号管理可用）")
		}
	} else {
		logger.Warn("platform.appKey 未配置：设备管理/租户同步/型号管理不可用")
	}

	if sc := cfg.Platform.Storage; sc.BaseURL != "" && sc.APIKeyID != "" {
		if err := platform.NewStorageClient(cfg).Ping(ctx); err != nil {
			logger.Warn("platform storage-gateway 不可达（文件上传将失败）", zap.Error(err))
		} else {
			logger.Info("platform storage-gateway ok（文件存储可用）")
		}
	}
}

// syncTenantPermCatalog 租户门户权限码注册表向平台同步（启动异步，目录动态化契约）：
// 商户端注册表（biz/tenantuser/permcatalog.go）是事实源，同步后注册码即可在本商户角色
// 配权中勾选。网络/服务端错误每 30s 重试至成功；4xx（scope 未配 tenant-role:syncPerms、
// 注册表格式被拒）为永久错误，记 error 后放弃——修复配置后重启服务重新对账。
func syncTenantPermCatalog(cfg *conf.Bootstrap) {
	defs := make([]platformsdk.TenantUserPermDef, 0, len(biztenantuser.Catalog))
	for _, d := range biztenantuser.Catalog {
		defs = append(defs, platformsdk.TenantUserPermDef{Code: d.Code, Group: d.Group})
	}
	client := platform.NewOpenAPIClient(cfg)
	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := client.SyncTenantUserPerms(ctx, defs)
		cancel()
		if err == nil {
			logger.Info("tenant perm catalog synced（注册码已对账，角色配权可勾选）", zap.Int("perms", len(defs)))
			return
		}
		var apiErr *platformsdk.Error
		if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
			logger.Error("tenant perm catalog sync 被拒绝（检查商户 scope tenant-role:syncPerms 与注册表格式），放弃重试",
				zap.Int("status", apiErr.HTTPStatus), zap.String("msg", apiErr.Msg))
			return
		}
		logger.Warn("tenant perm catalog sync failed, 30s 后重试", zap.Int("attempt", attempt), zap.Error(err))
		time.Sleep(30 * time.Second)
	}
}
