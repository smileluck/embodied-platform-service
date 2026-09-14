package merchant

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"time"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Usecase 商户领域用例
type Usecase struct {
	repo Repo
	logs APILogRepo
}

func NewUsecase(repo Repo, logs APILogRepo) *Usecase {
	return &Usecase{repo: repo, logs: logs}
}

// Create 创建商户：生成 appKey/appSecret，密钥只落哈希。
// 返回商户与明文 appSecret（仅此一次可见，请调用方妥善下发）。
func (uc *Usecase) Create(ctx context.Context, name, code, contactName, contactPhone, contactEmail, remark string) (*Merchant, string, error) {
	appKey, err := randomHex(8)
	if err != nil {
		return nil, "", err
	}
	secret, err := randomHex(32)
	if err != nil {
		return nil, "", err
	}
	m := &Merchant{
		Name: name, Code: code, AppKey: "mk_" + appKey,
		ContactName: contactName, ContactPhone: contactPhone, ContactEmail: contactEmail,
		Status: StatusEnabled, Remark: remark,
	}
	m.SetSecret(secret)
	// code/app_key 唯一性由数据库唯一索引兜底（冲突在仓储映射为 ErrDuplicateCode）
	if err := uc.repo.Create(ctx, m); err != nil {
		return nil, "", err
	}
	return m, secret, nil
}

// Update 更新商户基础资料（code 创建后不可改，不触碰密钥）
func (uc *Usecase) Update(ctx context.Context, id uint, name, contactName, contactPhone, contactEmail, remark string) error {
	m, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	m.Name = name
	m.ContactName = contactName
	m.ContactPhone = contactPhone
	m.ContactEmail = contactEmail
	m.Remark = remark
	return uc.repo.Update(ctx, m)
}

func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Merchant, error) {
	return uc.repo.Get(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Merchant, pagination.Page, error) {
	merchants, total, err := uc.repo.List(ctx, q, page, pageSize)
	return merchants, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// ResetSecret 重置商户密钥：生成新 secret 落哈希，返回明文一次（旧 secret 立即失效）
func (uc *Usecase) ResetSecret(ctx context.Context, id uint) (string, error) {
	m, err := uc.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	secret, err := randomHex(32)
	if err != nil {
		return "", err
	}
	m.SetSecret(secret)
	if err := uc.repo.Update(ctx, m); err != nil {
		return "", err
	}
	return secret, nil
}

// SetStatus 启用/禁用商户
func (uc *Usecase) SetStatus(ctx context.Context, id uint, status Status) error {
	m, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	m.Status = status
	return uc.repo.Update(ctx, m)
}

// VerifySign 开放 API 验签：按 appKey 查商户 → 启用校验 → HMAC-SHA256 验签。
// 签名串：METHOD + "\n" + path + "\n" + appKey + "\n" + timestamp + "\n" + nonce + "\n" + bodyHash；
// HMAC 密钥为派生密钥 SHA-256(AppKey + ":" + appSecret)（即存储哈希，客户端持明文 secret 可同样推导），
// 结果 hex 编码后常量时间比对。
// 时间戳偏差与 nonce 防重放由中间件负责，此处只管商户状态与签名。
// 返回商户供中间件注入请求上下文（商户存在但校验失败时也返回，便于调用日志归属）。
func (uc *Usecase) VerifySign(ctx context.Context, appKey, method, path, timestamp, nonce, bodyHash, sign string) (*Merchant, error) {
	m, err := uc.repo.FindByAppKey(ctx, appKey)
	if err != nil {
		return nil, err
	}
	if !m.Enabled() {
		return m, ErrMerchantDisabled
	}
	payload := method + "\n" + path + "\n" + appKey + "\n" + timestamp + "\n" + nonce + "\n" + bodyHash
	if !m.CheckSign(payload, sign) {
		return m, ErrInvalidSign
	}
	return m, nil
}

// CheckSign 常量时间校验签名：HMAC-SHA256(key=AppSecretHash, payload) 的 hex 与请求签名比对。
// 存储哈希对外不可见且不可逆，作为 HMAC 密钥材料等价于共享密钥。
func (m *Merchant) CheckSign(payload, sign string) bool {
	mac := hmac.New(sha256.New, []byte(m.AppSecretHash))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(sign)) == 1
}

// RecordAccess 记录一次开放 API 调用（透传到 APILogRepo，异步在 data 层实现）
func (uc *Usecase) RecordAccess(_ context.Context, l *APILog) {
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	uc.logs.Record(l)
}

// ListAPILogs 分页查询 API 调用日志
func (uc *Usecase) ListAPILogs(ctx context.Context, q APILogQuery, page, pageSize int) ([]*APILog, pagination.Page, error) {
	logs, total, err := uc.logs.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return logs, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// randomHex 生成 n 字节密码学随机的 hex 串
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
