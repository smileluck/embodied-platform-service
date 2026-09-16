package platform

import (
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

// NewOpenAPIClient 平台开放面客户端（商户 HMAC；设备域全部端点，签名/信封/分页由平台 SDK 封装）。
// wire provider：设备上下文经此访问 /open-api/v1，数据范围由平台按商户绑定租户集服务端收敛。
func NewOpenAPIClient(cfg *conf.Bootstrap) *sdk.Client {
	c := sdk.NewClient(cfg.Platform.BaseURL, cfg.Platform.AppKey, cfg.Platform.AppSecret)
	timeout := time.Duration(cfg.Platform.TimeoutSeconds) * time.Second
	if timeout > 0 {
		c.HTTP.Timeout = timeout
	}
	return c
}
