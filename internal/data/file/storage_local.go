package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
)

// localStorage 本地磁盘存储；不支持预签名，下载由后端代理流式输出
type localStorage struct {
	dir string
}

func newLocalStorage(dir string) bizfile.Storage {
	if dir == "" {
		dir = "./data/uploads"
	}
	return &localStorage{dir: dir}
}

func (s *localStorage) Driver() string { return "local" }

// path 拼接对象 key 的磁盘路径，前缀校验防路径穿越
func (s *localStorage) path(key string) (string, error) {
	root, err := filepath.Abs(s.dir)
	if err != nil {
		return "", err
	}
	p := filepath.Join(root, filepath.FromSlash(key))
	if p != root && !strings.HasPrefix(p, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid object key: %q", key)
	}
	return p, nil
}

func (s *localStorage) Put(ctx context.Context, key string, r io.Reader, _ int64, _ string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	// 先写临时文件再 rename，避免中断残留半截文件
	tmp, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, p)
}

func (s *localStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func (s *localStorage) Delete(_ context.Context, key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *localStorage) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", bizfile.ErrPresignUnsupported
}
