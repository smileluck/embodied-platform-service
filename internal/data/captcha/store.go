// Package captcha 图形验证码仓储 —— Redis 实现 base64Captcha.Store。
// 答案存 Redis（TTL 自动过期），多实例共享；校验一次性（GETDEL 原子取出即删）。
package captcha

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// captchaTTL 验证码有效期（与 biz 层注释保持一致）
const captchaTTL = 3 * time.Minute

func captchaKey(id string) string { return "captcha:" + id }

// Store Redis 验证码存储（实现 base64Captcha.Store）
type Store struct {
	client *redis.Client
}

func NewStore(client *redis.Client) *Store {
	return &Store{client: client}
}

// Set 存入验证码答案，TTL 到期自动失效
func (s *Store) Set(id string, value string) error {
	return s.client.Set(context.Background(), captchaKey(id), value, captchaTTL).Err()
}

// Get 读取答案；clear=true 时原子取出即删（一次性校验）
func (s *Store) Get(id string, clear bool) string {
	ctx := context.Background()
	key := captchaKey(id)
	if clear {
		val, err := s.client.GetDel(ctx, key).Result()
		if err != nil {
			return ""
		}
		return val
	}
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return ""
	}
	return val
}

// Verify 直接校验答案（clear 控制校验后是否删除）
func (s *Store) Verify(id, answer string, clear bool) bool {
	if answer == "" {
		return false
	}
	v := s.Get(id, clear)
	return v != "" && v == answer
}
