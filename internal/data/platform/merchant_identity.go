package platform

import (
	"context"
	"sync"

	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

// MerchantIdentity 本商户身份发现（开放面 /open-api/v1/ping 恒放行、无需 scope）：
// 用配置的商户 HMAC 凭证惰性解析本系统的平台商户 ID，进程内缓存（凭证绑定的
// 商户在运行期不变）。识别失败（平台不可达/响应异常）返回未知，调用方跳过
// 商户管理员投影的自愈（绝不因未知而误解绑）。
type MerchantIdentity struct {
	client *sdk.Client

	mu    sync.Mutex
	id    uint
	known bool
}

// NewMerchantIdentity 构造（wire provider，绑定 bizadmission.MerchantIdentity）
func NewMerchantIdentity(client *sdk.Client) *MerchantIdentity {
	return &MerchantIdentity{client: client}
}

// MerchantID 返回本商户的平台 ID；known=false 表示尚未识别成功（下次调用重试）
func (m *MerchantIdentity) MerchantID(ctx context.Context) (uint, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.known {
		return m.id, true
	}
	out, err := m.client.Ping(ctx)
	if err != nil {
		return 0, false
	}
	// ping 返回商户 VO 的 JSON map，id 为数字（json 反序列化为 float64）
	v, ok := out["id"].(float64)
	if !ok || v <= 0 {
		return 0, false
	}
	m.id, m.known = uint(v), true
	return m.id, true
}
