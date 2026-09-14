// Package logger 全局 zap 日志：区分开发/生产环境，支持按日期滚动落盘。
//
// 开发环境（mode=debug）：彩色可读 console 格式，默认 debug 级别，输出到控制台；
// 生产环境（mode=release）：JSON 格式，默认 info 级别，写入日志文件，
// 按日期滚动（dir/filename-2006-01-02.log），超期文件每日自动清理。
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config 运行日志配置（由 main 从 conf.Log 组装，避免 pkg 依赖 internal）
type Config struct {
	Dir        string // 日志文件目录，空则只输出控制台
	Filename   string // 日志文件名前缀（实际文件为 prefix-日期.log）
	Level      string // 最低级别：debug|info|warn|error，空则 debug 环境=debug，release=info
	MaxAgeDays int    // 日志文件保留天数，超期自动清理；0 永久保留
	Console    bool   // 是否同时输出到控制台（release 默认 false，debug 默认 true）
}

var L *zap.Logger

func Init(mode string, cfg Config) error {
	dev := mode != "release"

	level := zapcore.InfoLevel
	if dev {
		level = zapcore.DebugLevel
	}
	if cfg.Level != "" {
		var l zapcore.Level
		if err := l.UnmarshalText([]byte(cfg.Level)); err != nil {
			return err
		}
		level = l
	}

	var cores []zapcore.Core

	// 控制台输出：dev 用彩色可读格式；release 如需 console 则去彩色
	if cfg.Console || dev {
		encCfg := zap.NewDevelopmentEncoderConfig()
		if !dev {
			encCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		}
		cores = append(cores, zapcore.NewCore(zapcore.NewConsoleEncoder(encCfg), zapcore.Lock(os.Stdout), level))
	}

	// 文件输出：JSON 格式，按日期滚动
	if cfg.Dir != "" {
		encCfg := zap.NewProductionEncoderConfig()
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		w := newDateWriter(cfg.Dir, cfg.Filename, cfg.MaxAgeDays)
		cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(encCfg), zapcore.AddSync(w), level))
	}

	opts := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(1)} // skip 包级封装函数
	if dev {
		opts = append(opts, zap.Development())
	}
	L = zap.New(zapcore.NewTee(cores...), opts...)
	return nil
}

// Sync 刷新缓冲日志，进程退出前调用
func Sync() { _ = L.Sync() }

func Debug(msg string, fields ...zap.Field) { L.Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)  { L.Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { L.Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field) { L.Error(msg, fields...) }
