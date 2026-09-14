package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dateLayout 日志文件按天滚动的日期格式（文件名后缀）
const dateLayout = "2006-01-02"

// dateWriter 按日期滚动的文件 writer：写入 dir/prefix-<日期>.log，
// 跨日自动切换文件句柄，并清理 maxAgeDays 之前的旧文件。
type dateWriter struct {
	dir        string
	prefix     string
	maxAgeDays int

	mu     sync.Mutex
	day    string
	file   *os.File
	lastGC string // 上次执行旧文件清理的日期，避免每次写入都扫目录
}

func newDateWriter(dir, prefix string, maxAgeDays int) *dateWriter {
	return &dateWriter{dir: dir, prefix: prefix, maxAgeDays: maxAgeDays}
}

// Write 实现 io.Writer（zapcore.AddSync 传入后同时满足 WriteSyncer，Sync 由文件句柄承担）
func (w *dateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.rotateLocked(); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

// Sync 刷新当前文件到磁盘
func (w *dateWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

// Close 关闭当前文件句柄
func (w *dateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// filename 返回指定日期对应的日志文件路径
func (w *dateWriter) filename(day string) string {
	return filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, day))
}

// rotateLocked 按当前日期打开/切换日志文件，跨日时顺带清理过期文件
func (w *dateWriter) rotateLocked() error {
	day := time.Now().Format(dateLayout)
	if w.file != nil && w.day == day {
		return nil
	}
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(w.filename(day), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.day = day
	// 每个新的一天（或首次打开）清理一次过期文件
	if w.lastGC != day {
		w.lastGC = day
		w.cleanOldLocked()
	}
	return nil
}

// cleanOldLocked 删除 dir 下 prefix-*.log 中日期早于 maxAgeDays 天前的文件
func (w *dateWriter) cleanOldLocked() {
	if w.maxAgeDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -w.maxAgeDays).Format(dateLayout)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	prefix := w.prefix + "-"
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		day := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".log")
		// 非法命名跳过；日期串定长（2006-01-02）字典序即时间序，早于 cutoff 则过期删除
		if _, err := time.Parse(dateLayout, day); err != nil {
			continue
		}
		if day < cutoff {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}
