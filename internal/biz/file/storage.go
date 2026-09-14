// Package file 文件管理限界上下文 —— 领域层
package file

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrPresignUnsupported 该存储后端不支持预签名 URL（如本地存储，走后端代理下载）
var ErrPresignUnsupported = errors.New("该存储后端不支持预签名")

// ErrDriverUnavailable 文件记录所属的存储后端当前未配置/不可用
var ErrDriverUnavailable = errors.New("文件所属的存储后端未配置，无法访问")

// Storage 对象存储抽象：Put/Get/Delete 按对象 key 操作；
// PresignGet 仅云存储支持（签发短时效下载 URL），本地存储返回 ErrPresignUnsupported
type Storage interface {
	// Driver 后端标识（local | oss | cos | tos | minio），与文件记录落库的 driver 对应
	Driver() string
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, expire time.Duration) (string, error)
}

// StorageManager 多驱动注册表：写路径用 Current()（配置的当前后端），
// 读/删路径用 For(driver) 按文件记录落库时的后端解析 —— 存储后端升级后旧文件仍可访问的关键
type StorageManager struct {
	backends map[string]Storage
	current  string
}

// NewStorageManager 注册各后端并指定当前写入驱动；当前驱动未注册时返回 ErrDriverUnavailable
func NewStorageManager(current string, backends ...Storage) (*StorageManager, error) {
	m := &StorageManager{backends: make(map[string]Storage, len(backends)), current: current}
	for _, b := range backends {
		if b == nil {
			continue
		}
		m.backends[b.Driver()] = b
	}
	if _, ok := m.backends[current]; !ok {
		return nil, ErrDriverUnavailable
	}
	return m, nil
}

// Current 当前写入后端（上传用）
func (m *StorageManager) Current() Storage { return m.backends[m.current] }

// For 按驱动标识取后端（下载/删除用）；未注册返回 ErrDriverUnavailable
func (m *StorageManager) For(driver string) (Storage, error) {
	if b, ok := m.backends[driver]; ok {
		return b, nil
	}
	return nil, ErrDriverUnavailable
}
