// SmileX-Admin-Gin 服务入口
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
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
