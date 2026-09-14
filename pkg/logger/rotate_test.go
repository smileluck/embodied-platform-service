package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDateWriterWriteAndRotate(t *testing.T) {
	dir := t.TempDir()
	w := newDateWriter(dir, "app", 30)
	defer w.Close()

	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	today := time.Now().Format(dateLayout)
	data, err := os.ReadFile(filepath.Join(dir, "app-"+today+".log"))
	if err != nil {
		t.Fatalf("read today file: %v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("unexpected content: %q", data)
	}

	// 模拟跨日：把当前日期回拨一天，触发 rotate 切换文件
	w.mu.Lock()
	w.day = time.Now().AddDate(0, 0, -1).Format(dateLayout)
	w.mu.Unlock()
	if _, err := w.Write([]byte("next day\n")); err != nil {
		t.Fatalf("write after rotate: %v", err)
	}
	if got := w.day; got != today {
		t.Fatalf("day not rotated, got %s want %s", got, today)
	}
	data, err = os.ReadFile(filepath.Join(dir, "app-"+today+".log"))
	if err != nil {
		t.Fatalf("read rotated file: %v", err)
	}
	if string(data) != "hello\nnext day\n" {
		t.Fatalf("unexpected rotated content: %q", data)
	}
}

func TestDateWriterCleanOld(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().AddDate(0, 0, -40).Format(dateLayout)
	recent := time.Now().AddDate(0, 0, -10).Format(dateLayout)
	for _, name := range []string{
		"app-" + old + ".log",    // 超期，应删除
		"app-" + recent + ".log", // 未超期，保留
		"app-not-a-date.log",     // 非法命名，保留
		"other-" + old + ".log",  // 其他前缀，保留
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	w := newDateWriter(dir, "app", 30)
	defer w.Close()
	if _, err := w.Write([]byte("trigger\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "app-"+old+".log")); !os.IsNotExist(err) {
		t.Fatalf("old file should be removed, stat err=%v", err)
	}
	for _, name := range []string{"app-" + recent + ".log", "app-not-a-date.log", "other-" + old + ".log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s should be kept: %v", name, err)
		}
	}
}
