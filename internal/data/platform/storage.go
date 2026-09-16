package platform

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// StorageClient storage-gateway OpenAPI 客户端（默认 :27091，前缀 /openapi/storage/v1，
// 鉴权 Authorization: Bearer {apiKeyId}:{apiSecret}）。文件上传/下载统一走预签名：
// 云后端返回原生预签名 URL，local 后端返回网关代理 URL，调用方无需区分。
type StorageClient struct {
	baseURL   string
	apiKeyID  string
	apiSecret string
	hc        *http.Client
}

// NewStorageClient 构造（wire provider）
func NewStorageClient(cfg *conf.Bootstrap) *StorageClient {
	timeout := time.Duration(cfg.Platform.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &StorageClient{
		baseURL:   cfg.Platform.Storage.BaseURL,
		apiKeyID:  cfg.Platform.Storage.APIKeyID,
		apiSecret: cfg.Platform.Storage.APISecret,
		hc:        &http.Client{Timeout: timeout},
	}
}

func (c *StorageClient) auth() string {
	return c.apiKeyID + ":" + c.apiSecret
}

// Ping 网关健康检查（GET /healthz，无需认证）
func (c *StorageClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("storage gateway unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("storage gateway healthz: http %d", resp.StatusCode)
	}
	return nil
}

// presignResp 预签名响应
type presignResp struct {
	URL           string `json:"url"`
	Method        string `json:"method"`
	ObjectKey     string `json:"object_key"`
	ExpireMinutes int32  `json:"expire_minutes"`
}

// PresignPut 预签名上传（后端中转上传用：先取 PUT URL，再由后端写入对象字节）
func (c *StorageClient) PresignPut(ctx context.Context, bucket, key, contentType string, size int64) (string, error) {
	body := map[string]any{
		"bucket":         bucket,
		"key":            key,
		"size":           size,
		"content_type":   contentType,
		"expire_minutes": 15,
	}
	var out presignResp
	if err := do(ctx, c.hc, http.MethodPost, c.baseURL+"/openapi/storage/v1/uploads/presign", c.auth(), body, &out); err != nil {
		return "", err
	}
	return out.URL, nil
}

// CompleteUpload 上传完成登记（网关 Stat 校验对象存在并扣配额）
func (c *StorageClient) CompleteUpload(ctx context.Context, bucket, key, contentType string) error {
	body := map[string]string{
		"bucket":       bucket,
		"key":          key,
		"content_type": contentType,
	}
	return do(ctx, c.hc, http.MethodPost, c.baseURL+"/openapi/storage/v1/uploads/complete", c.auth(), body, nil)
}

// PresignGet 预签名下载 URL（expireMinutes 默认 5、上限 60；要求对象已在网关元数据登记）
func (c *StorageClient) PresignGet(ctx context.Context, bucket, key string, expireMinutes int) (string, error) {
	if expireMinutes <= 0 {
		expireMinutes = 5
	}
	endpoint := fmt.Sprintf("%s/openapi/storage/v1/files/%s/%s/url?expire=%d", c.baseURL, bucket, key, expireMinutes)
	var out presignResp
	if err := do(ctx, c.hc, http.MethodGet, endpoint, c.auth(), nil, &out); err != nil {
		return "", err
	}
	return out.URL, nil
}

// Delete 删除对象（网关侧软删元数据）
func (c *StorageClient) Delete(ctx context.Context, bucket, key string) error {
	endpoint := fmt.Sprintf("%s/openapi/storage/v1/files/%s/%s", c.baseURL, bucket, key)
	return do(ctx, c.hc, http.MethodDelete, endpoint, c.auth(), nil, nil)
}

// PutBytes 后端中转上传：presign → PUT 原始字节 → complete
func (c *StorageClient) PutBytes(ctx context.Context, bucket, key, contentType string, data []byte) error {
	uploadURL, err := c.PresignPut(ctx, bucket, key, contentType, int64(len(data)))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("platform storage: build put request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("platform storage: put object: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("platform storage: put object: http %d", resp.StatusCode)
	}
	return c.CompleteUpload(ctx, bucket, key, contentType)
}
