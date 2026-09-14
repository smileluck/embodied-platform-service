package file

import (
	"context"
	"io"
	"net/url"
	"time"

	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// minioStorage 自定义 S3 兼容存储（JuiceFS + MinIO 场景：对象经 MinIO 落 JuiceFS 卷）
type minioStorage struct {
	client *minio.Client
	bucket string
	prefix string
}

func newMinIOStorage(c conf.MinIOStorage) (bizfile.Storage, error) {
	client, err := minio.New(c.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""),
		Secure: c.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &minioStorage{client: client, bucket: c.Bucket, prefix: c.Prefix}, nil
}

func (s *minioStorage) Driver() string { return "minio" }

func (s *minioStorage) key(k string) string { return s.prefix + k }

func (s *minioStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, s.key(key), r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *minioStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, s.key(key), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// minio.Object 惰性请求：先 Stat 触发一次网络往返，提前暴露不存在等错误
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, err
	}
	return obj, nil
}

func (s *minioStorage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, s.key(key), minio.RemoveObjectOptions{})
}

func (s *minioStorage) PresignGet(ctx context.Context, key string, expire time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, s.key(key), expire, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
