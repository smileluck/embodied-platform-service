package file

import (
	"context"
	"io"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// ossStorage 阿里云 OSS
type ossStorage struct {
	bucket *oss.Bucket
	prefix string
}

func newOSSStorage(c conf.OSSStorage) (bizfile.Storage, error) {
	client, err := oss.New(c.Endpoint, c.AccessKeyID, c.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	bucket, err := client.Bucket(c.Bucket)
	if err != nil {
		return nil, err
	}
	return &ossStorage{bucket: bucket, prefix: c.Prefix}, nil
}

func (s *ossStorage) Driver() string { return "oss" }

func (s *ossStorage) key(k string) string { return s.prefix + k }

func (s *ossStorage) Put(_ context.Context, key string, r io.Reader, _ int64, contentType string) error {
	return s.bucket.PutObject(s.key(key), r, oss.ContentType(contentType))
}

func (s *ossStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	return s.bucket.GetObject(s.key(key))
}

func (s *ossStorage) Delete(_ context.Context, key string) error {
	return s.bucket.DeleteObject(s.key(key))
}

func (s *ossStorage) PresignGet(_ context.Context, key string, expire time.Duration) (string, error) {
	return s.bucket.SignURL(s.key(key), oss.HTTPGet, int64(expire.Seconds()))
}
