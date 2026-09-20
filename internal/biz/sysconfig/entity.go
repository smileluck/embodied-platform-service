// Package sysconfig 系统参数限界上下文 —— 领域层。
// 运行时可调的业务参数（区别于 config.yaml 的启动配置）；
// 读走进程内缓存（短 TTL 回源），供各模块经 Get/GetDefault 消费。
package sysconfig

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"
)

// 哨兵错误
var (
	ErrNotFound   = errors.New("参数不存在")
	ErrKeyExists  = errors.New("参数键已存在，请更换")
	ErrBadValue   = errors.New("参数值与类型不匹配")
	ErrKeyInvalid = errors.New("参数键只能包含字母、数字、下划线、点")
)

// ValueType 参数值类型
type ValueType string

const (
	TypeString ValueType = "string"
	TypeNumber ValueType = "number"
	TypeBool   ValueType = "bool"
)

// Config 系统参数
type Config struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        ValueType `json:"type"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Repo 仓储接口
type Repo interface {
	Create(ctx context.Context, c *Config) error
	Update(ctx context.Context, c *Config) error
	Delete(ctx context.Context, key string) error
	Find(ctx context.Context, key string) (*Config, error)
	List(ctx context.Context, keyword string) ([]*Config, error)
}

// 内置参数定义（启动幂等播种；删除后重启自愈）
var builtin = []Config{
	{Key: "password.minLength", Value: "6", Type: TypeNumber, Description: "密码最小长度（6-20）"},
	{Key: "page.sizeMax", Value: "100", Type: TypeNumber, Description: "列表分页单页上限"},
	{Key: "agent.chatRatePerMin", Value: "20", Type: TypeNumber, Description: "Agent 对话每用户每分钟次数上限"},
}

// Usecase 系统参数用例：读走 30s 进程内缓存（单机语义，与 RBAC 缓存一致）
type Usecase struct {
	repo Repo

	mu       sync.RWMutex
	cached   map[string]Config
	loadedAt time.Time
}

func NewUsecase(repo Repo) *Usecase {
	return &Usecase{repo: repo, cached: map[string]Config{}}
}

// EnsureBuiltin 启动播种内置参数（已存在则跳过，不覆盖用户修改）
func (uc *Usecase) EnsureBuiltin(ctx context.Context) error {
	for _, b := range builtin {
		if _, err := uc.repo.Find(ctx, b.Key); err == nil {
			continue
		}
		if err := uc.repo.Create(ctx, &b); err != nil {
			return err
		}
	}
	return nil
}

// validateValue 校验值与类型匹配
func validateValue(t ValueType, v string) error {
	switch t {
	case TypeNumber:
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return ErrBadValue
		}
	case TypeBool:
		if v != "true" && v != "false" {
			return ErrBadValue
		}
	}
	return nil
}

func (uc *Usecase) Create(ctx context.Context, c *Config) (*Config, error) {
	if !validKey(c.Key) {
		return nil, ErrKeyInvalid
	}
	if err := validateValue(c.Type, c.Value); err != nil {
		return nil, err
	}
	if _, err := uc.repo.Find(ctx, c.Key); err == nil {
		return nil, ErrKeyExists
	}
	if err := uc.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	uc.invalidate()
	return c, nil
}

func (uc *Usecase) Update(ctx context.Context, key, value string) (*Config, error) {
	c, err := uc.repo.Find(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := validateValue(c.Type, value); err != nil {
		return nil, err
	}
	c.Value = value
	if err := uc.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	uc.invalidate()
	return c, nil
}

func (uc *Usecase) Delete(ctx context.Context, key string) error {
	if err := uc.repo.Delete(ctx, key); err != nil {
		return err
	}
	uc.invalidate()
	return nil
}

func (uc *Usecase) List(ctx context.Context, keyword string) ([]*Config, error) {
	return uc.repo.List(ctx, keyword)
}

// GetDefault 消费入口：读缓存值（未配置/类型不符回退默认值；缓存 30s 过期回源）
func (uc *Usecase) GetDefault(ctx context.Context, key string, def string) string {
	if c, ok := uc.snapshot(ctx)[key]; ok {
		return c.Value
	}
	return def
}

// GetIntDefault 整型读取（解析失败回退默认）
func (uc *Usecase) GetIntDefault(ctx context.Context, key string, def int) int {
	if v, err := strconv.Atoi(uc.GetDefault(ctx, key, strconv.Itoa(def))); err == nil && v > 0 {
		return v
	}
	return def
}

const cacheTTL = 30 * time.Second

// snapshot 取全量缓存（过期回源重建；回源失败沿用旧值）
func (uc *Usecase) snapshot(ctx context.Context) map[string]Config {
	uc.mu.RLock()
	if time.Since(uc.loadedAt) < cacheTTL && uc.loadedAt != (time.Time{}) {
		defer uc.mu.RUnlock()
		return uc.cached
	}
	uc.mu.RUnlock()

	uc.mu.Lock()
	defer uc.mu.Unlock()
	if time.Since(uc.loadedAt) < cacheTTL && uc.loadedAt != (time.Time{}) {
		return uc.cached
	}
	list, err := uc.repo.List(ctx, "")
	if err != nil || len(list) == 0 {
		if len(uc.cached) > 0 {
			return uc.cached // 回源失败沿用旧缓存
		}
		if err != nil {
			return map[string]Config{}
		}
	}
	next := make(map[string]Config, len(list))
	for _, c := range list {
		next[c.Key] = *c
	}
	uc.cached = next
	uc.loadedAt = time.Now()
	return next
}

func (uc *Usecase) invalidate() {
	uc.mu.Lock()
	uc.cached = map[string]Config{}
	uc.loadedAt = time.Time{}
	uc.mu.Unlock()
}

// validKey 键格式：字母/数字/下划线/点，1~64
func validKey(k string) bool {
	if len(k) == 0 || len(k) > 64 {
		return false
	}
	for _, r := range k {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
