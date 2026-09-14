// Package file 文件元数据仓储 GORM 实现与各存储后端适配
package file

import (
	"fmt"

	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// NewStorageManager 按配置构造全部已配置完整的存储后端：
// local 总是可用；云存储凭据齐全才注册。当前 driver 未注册时返回错误（启动即暴露配置问题）。
// 读/删按文件记录的 driver 解析后端，因此升级存储后端后旧文件仍可访问（旧后端配置需保留）。
func NewStorageManager(c *conf.Bootstrap) (*bizfile.StorageManager, error) {
	backends := []bizfile.Storage{newLocalStorage(c.Storage.Local.Dir)}

	if o := c.Storage.OSS; o.Endpoint != "" && o.Bucket != "" && o.AccessKeyID != "" && o.AccessKeySecret != "" {
		b, err := newOSSStorage(o)
		if err != nil {
			return nil, fmt.Errorf("init oss storage: %w", err)
		}
		backends = append(backends, b)
	}
	if o := c.Storage.COS; o.Region != "" && o.Bucket != "" && o.SecretID != "" && o.SecretKey != "" {
		backends = append(backends, newCOSStorage(o))
	}
	if o := c.Storage.TOS; o.Endpoint != "" && o.Region != "" && o.Bucket != "" && o.AccessKey != "" && o.SecretKey != "" {
		b, err := newTOSStorage(o)
		if err != nil {
			return nil, fmt.Errorf("init tos storage: %w", err)
		}
		backends = append(backends, b)
	}
	if o := c.Storage.MinIO; o.Endpoint != "" && o.Bucket != "" && o.AccessKey != "" && o.SecretKey != "" {
		b, err := newMinIOStorage(o)
		if err != nil {
			return nil, fmt.Errorf("init minio storage: %w", err)
		}
		backends = append(backends, b)
	}

	return bizfile.NewStorageManager(c.Storage.Driver, backends...)
}
