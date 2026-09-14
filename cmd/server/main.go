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
