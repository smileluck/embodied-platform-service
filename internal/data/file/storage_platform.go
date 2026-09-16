// 平台存储驱动：所有对象存取经 embodied-platform storage-gateway（API Key 鉴权，
// 预签名上传/下载统一入口——云后端为原生预签名 URL，local 后端为网关代理 URL）。
package file

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
)

// platformStorage 平台 storage-gateway 存储后端
type platformStorage struct {
	client *platform.StorageClient
	bucket string
}

// newPlatformStorage 构造；baseUrl 未配置时返回 nil（不注册该后端）
func newPlatformStorage(client *platform.StorageClient, bucket string) *platformStorage {
	if client == nil {
		return nil
	}
	return &platformStorage{client: client, bucket: bucket}
}

func (s *platformStorage) Driver() string { return "platform" }

// Put 后端中转上传：读全量字节 → presign → PUT → complete。
// 上限由业务层限制（storage.maxSizeMB，默认 20MB），内存缓冲可接受。
func (s *platformStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if size < 0 || size > 512<<20 {
		// 防御：超过 512MB 拒绝中转（正常上限远低于此）
		return fmt.Errorf("platform storage: object too large to proxy (%d bytes)", size)
	}
	data, err := io.ReadAll(io.LimitReader(r, size))
	if err != nil {
		return fmt.Errorf("platform storage: read body: %w", err)
	}
	return s.client.PutBytes(ctx, s.bucket, key, contentType, data)
}

// Get 拉取对象内容（网关预签名 GET 后回流；主要供内部消费，常规下载走 PresignGet 302）
func (s *platformStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	url, err := s.PresignGet(ctx, key, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("platform storage: get object: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("platform storage: get object: http %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (s *platformStorage) Delete(ctx context.Context, key string) error {
	return s.client.Delete(ctx, s.bucket, key)
}

// PresignGet 预签名下载 URL（expire 分钟上限 60，超出收敛到 60）
func (s *platformStorage) PresignGet(ctx context.Context, key string, expire time.Duration) (string, error) {
	mins := int(expire / time.Minute)
	if mins <= 0 {
		mins = 5
	}
	if mins > 60 {
		mins = 60
	}
	return s.client.PresignGet(ctx, s.bucket, key, mins)
}

// 接口约束编译期校验
var _ bizfile.Storage = (*platformStorage)(nil)
