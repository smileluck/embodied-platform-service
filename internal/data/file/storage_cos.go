package file

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/tencentyun/cos-go-sdk-v5"
)

// cosStorage 腾讯云 COS
type cosStorage struct {
	client *cos.Client
	cfg    conf.COSStorage // 预签名需显式提供 SecretID/SecretKey
}

func newCOSStorage(c conf.COSStorage) bizfile.Storage {
	u, _ := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", c.Bucket, c.Region))
	return &cosStorage{cfg: c, client: cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{SecretID: c.SecretID, SecretKey: c.SecretKey},
	})}
}

func (s *cosStorage) Driver() string { return "cos" }

func (s *cosStorage) key(k string) string { return s.cfg.Prefix + k }

func (s *cosStorage) Put(ctx context.Context, key string, r io.Reader, _ int64, contentType string) error {
	_, err := s.client.Object.Put(ctx, s.key(key), r, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: contentType},
	})
	return err
}

func (s *cosStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := s.client.Object.Get(ctx, s.key(key), nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (s *cosStorage) Delete(ctx context.Context, key string) error {
	_, err := s.client.Object.Delete(ctx, s.key(key))
	return err
}

func (s *cosStorage) PresignGet(ctx context.Context, key string, expire time.Duration) (string, error) {
	u, err := s.client.Object.GetPresignedURL(ctx, http.MethodGet, s.key(key), s.cfg.SecretID, s.cfg.SecretKey, expire, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
