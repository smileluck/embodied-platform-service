// Package file 文件元数据仓储 GORM 实现与存储后端适配。
//
// 存储后端只有两个：
//   - platform：embodied-platform storage-gateway（默认写入后端，配置 platform.storage 后启用）
//   - local：本地磁盘（平台存储未配置时的降级写入后端；同时承载历史存量文件的读取）
//
// 读/删按文件记录落库的 driver 解析后端，旧文件在新后端上线后仍可访问。
package file

import (
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
)

// NewStorageManager 构造存储后端注册表：
// driver 显式配置时以配置为准；留空时自动选择——平台存储配置齐全选 platform，否则降级 local。
func NewStorageManager(c *conf.Bootstrap, storageClient *platform.StorageClient) (*bizfile.StorageManager, error) {
	backends := []bizfile.Storage{newLocalStorage(c.Storage.Local.Dir)}

	platformReady := c.Platform.Storage.BaseURL != "" && c.Platform.Storage.APIKeyID != "" && c.Platform.Storage.APISecret != ""
	current := "local"
	if c.Storage.Driver != "" {
		current = c.Storage.Driver
	} else if platformReady {
		current = "platform"
	}

	if platformReady {
		if b := newPlatformStorage(storageClient, c.Platform.Storage.Bucket); b != nil {
			backends = append(backends, b)
		}
	} else if current == "platform" {
		logger.Warn("platform storage not configured, falling back to local dir " +
			"(文件不会进入平台存储；生产环境请配置 platform.storage)")
		current = "local"
	}

	return bizfile.NewStorageManager(current, backends...)
}
