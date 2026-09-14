package file

import (
	"context"
	"io"
	"time"

	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"
)

// tosStorage 火山引擎 TOS
type tosStorage struct {
	client *tos.ClientV2
	bucket string
	prefix string
}

func newTOSStorage(c conf.TOSStorage) (bizfile.Storage, error) {
	client, err := tos.NewClientV2(c.Endpoint,
		tos.WithRegion(c.Region),
		tos.WithCredentials(tos.NewStaticCredentials(c.AccessKey, c.SecretKey)),
	)
	if err != nil {
		return nil, err
	}
	return &tosStorage{client: client, bucket: c.Bucket, prefix: c.Prefix}, nil
}

func (s *tosStorage) Driver() string { return "tos" }

func (s *tosStorage) key(k string) string { return s.prefix + k }

func (s *tosStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObjectV2(ctx, &tos.PutObjectV2Input{
		PutObjectBasicInput: tos.PutObjectBasicInput{
			Bucket:        s.bucket,
			Key:           s.key(key),
			ContentType:   contentType,
			ContentLength: size,
		},
		Content: r,
	})
	return err
}

func (s *tosStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObjectV2(ctx, &tos.GetObjectV2Input{Bucket: s.bucket, Key: s.key(key)})
	if err != nil {
		return nil, err
	}
	return out.Content, nil
}

func (s *tosStorage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObjectV2(ctx, &tos.DeleteObjectV2Input{Bucket: s.bucket, Key: s.key(key)})
	return err
}

func (s *tosStorage) PresignGet(_ context.Context, key string, expire time.Duration) (string, error) {
	out, err := s.client.PreSignedURL(&tos.PreSignedURLInput{
		HTTPMethod: enum.HttpMethodGet,
		Bucket:     s.bucket,
		Key:        s.key(key),
		Expires:    int64(expire.Seconds()),
	})
	if err != nil {
		return "", err
	}
	return out.SignedUrl, nil
}
