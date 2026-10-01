package mcp

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// 状态值（与全局约定一致：1 启用 0 禁用）
const (
	StatusDisabled = 0
	StatusEnabled  = 1
)

// codeRe 编码约束：字母数字开头，可含下划线/中划线，禁「:」（Agent 以 mcp:<code>:<tool> 引用，避免解析歧义）
var codeRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// Usecase MCP 领域用例：配置 CRUD + 连通测试 + 工具发现/调用（供智能体 function calling）
type Usecase struct {
	repo    Repo
	crypto  *Crypto
	manager *Manager
}

func NewUsecase(repo Repo, cfg *conf.Bootstrap) *Usecase {
	return &Usecase{repo: repo, crypto: NewCrypto(cfg.MCP.CryptoKey, cfg.JWT.Secret), manager: NewManager()}
}

// ServerInput 写入参数（Token 明文仅在写入链路出现；更新时留空=保持不变）
type ServerInput struct {
	Name      string
	Code      string
	Transport Transport
	BaseURL   string
	Headers   []Header
	Token     string
	Remark    string
	Status    int
}

func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return ErrInvalidURL
	}
	return nil
}

func (uc *Usecase) Create(ctx context.Context, in ServerInput) (*Server, error) {
	if !codeRe.MatchString(in.Code) {
		return nil, ErrInvalidCode
	}
	if !ValidTransport(string(in.Transport)) {
		return nil, ErrInvalidTransport
	}
	if err := checkURL(in.BaseURL); err != nil {
		return nil, err
	}
	if _, err := uc.repo.FindByCode(ctx, in.Code); err == nil {
		return nil, ErrCodeExists
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if _, err := uc.repo.FindByName(ctx, in.Name); err == nil {
		return nil, ErrNameExists
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	enc, err := uc.crypto.Encrypt(in.Token)
	if err != nil {
		return nil, err
	}
	s := &Server{
		Name: in.Name, Code: in.Code, Transport: in.Transport, BaseURL: strings.TrimRight(in.BaseURL, "/"),
		Headers: in.Headers, TokenEnc: enc, TokenMask: MaskToken(in.Token),
		Remark: in.Remark, Status: in.Status,
	}
	if err := uc.repo.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (uc *Usecase) Update(ctx context.Context, id uint, in ServerInput) error {
	s, err := uc.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	if !codeRe.MatchString(in.Code) {
		return ErrInvalidCode
	}
	if !ValidTransport(string(in.Transport)) {
		return ErrInvalidTransport
	}
	if err := checkURL(in.BaseURL); err != nil {
		return err
	}
	if in.Code != s.Code {
		// 改码会使既有 mcp:<旧码>: 引用失效：被绑定时禁改
		if n, err := uc.repo.CountAgentsUsing(ctx, s.Code); err != nil {
			return err
		} else if n > 0 {
			return ErrServerInUse
		}
		if _, err := uc.repo.FindByCode(ctx, in.Code); err == nil {
			return ErrCodeExists
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		s.Code = in.Code
	}
	if in.Name != s.Name {
		if _, err := uc.repo.FindByName(ctx, in.Name); err == nil {
			return ErrNameExists
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	s.Name, s.Transport = in.Name, in.Transport
	s.BaseURL, s.Headers, s.Remark, s.Status = strings.TrimRight(in.BaseURL, "/"), in.Headers, in.Remark, in.Status
	if in.Token != "" {
		enc, err := uc.crypto.Encrypt(in.Token)
		if err != nil {
			return err
		}
		s.TokenEnc, s.TokenMask = enc, MaskToken(in.Token)
	}
	if err := uc.repo.Update(ctx, s); err != nil {
		return err
	}
	uc.manager.drop(s.Code) // 配置变更：会话与工具缓存失效
	return nil
}

func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	s, err := uc.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	if n, err := uc.repo.CountAgentsUsing(ctx, s.Code); err != nil {
		return err
	} else if n > 0 {
		return ErrServerInUse
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.manager.drop(s.Code)
	return nil
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Server, error) {
	return uc.repo.Find(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Server, pagination.Page, error) {
	list, total, err := uc.repo.List(ctx, q, page, pageSize)
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// TestResult 连通测试结果（失败也是有效结果：OK=false 携带原因，不作为接口错误）
type TestResult struct {
	OK              bool       `json:"ok"`
	ServerName      string     `json:"server_name"`
	ServerVersion   string     `json:"server_version"`
	ProtocolVersion string     `json:"protocol_version"`
	Tools           []ToolInfo `json:"tools"`
	LatencyMS       int64      `json:"latency_ms"`
	Error           string     `json:"error,omitempty"`
}

// Test 连通测试：全新握手（不复用会话），initialize + tools/list
func (uc *Usecase) Test(ctx context.Context, id uint) (*TestResult, error) {
	s, err := uc.repo.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	token, err := uc.crypto.Decrypt(s.TokenEnc)
	if err != nil {
		return nil, err
	}
	cli := NewClient(ClientConfig{Transport: s.Transport, BaseURL: s.BaseURL, Headers: s.Headers, Token: token})
	defer cli.Close()

	start := time.Now()
	tctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	info, proto, err := cli.Handshake(tctx)
	if err != nil {
		return &TestResult{OK: false, LatencyMS: msSince(start), Error: err.Error()}, nil
	}
	tools, err := cli.ListTools(tctx)
	if err != nil {
		return &TestResult{OK: false, LatencyMS: msSince(start), Error: err.Error()}, nil
	}
	// 测试通过顺手刷新工具缓存（chat 下发与发现接口受益）
	uc.manager.cacheTools(s.Code, tools, *info)
	return &TestResult{
		OK: true, ServerName: info.Name, ServerVersion: info.Version,
		ProtocolVersion: proto, Tools: tools, LatencyMS: msSince(start),
	}, nil
}

func msSince(start time.Time) int64 { return time.Since(start).Milliseconds() }

// ---- 智能体接入（bizagent.MCPToolSource 实现） ----

// ServerRef 启用服务的轻量引用（Agent 表单分组用）
type ServerRef struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// ToolDefs 指定服务的工具定义（tools/list，带 5 分钟缓存；连接失败透传哨兵错误由调用方降级）
func (uc *Usecase) ToolDefs(ctx context.Context, serverCode string) ([]ToolInfo, error) {
	if tools, _, ok := uc.manager.cachedTools(serverCode); ok {
		return tools, nil
	}
	s, err := uc.repo.FindByCode(ctx, serverCode)
	if err != nil {
		return nil, err
	}
	cli, err := uc.sessionFor(ctx, s)
	if err != nil {
		return nil, err
	}
	tools, err := cli.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	uc.manager.cacheTools(s.Code, tools, ServerInfo{})
	return tools, nil
}

// CallTool 执行远程工具（单次 60s 上限：MCP 工具普遍为网络操作，宽于本地工具的 10s）
func (uc *Usecase) CallTool(ctx context.Context, serverCode, tool, argsJSON string) (string, error) {
	s, err := uc.repo.FindByCode(ctx, serverCode)
	if err != nil {
		return "", err
	}
	if s.Status != StatusEnabled {
		return "", ErrServerDisabled
	}
	cli, err := uc.sessionFor(ctx, s)
	if err != nil {
		return "", err
	}
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return cli.CallTool(cctx, tool, argsJSON)
}

// EnabledServers 全部启用服务（Agent 表单工具分组数据源）
func (uc *Usecase) EnabledServers(ctx context.Context) ([]ServerRef, error) {
	list, err := uc.repo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	refs := make([]ServerRef, 0, len(list))
	for _, s := range list {
		refs = append(refs, ServerRef{ID: s.ID, Code: s.Code, Name: s.Name})
	}
	return refs, nil
}

// CheckServer 校验编码存在且启用（Agent 绑定引用时的强校验）
func (uc *Usecase) CheckServer(ctx context.Context, serverCode string) error {
	s, err := uc.repo.FindByCode(ctx, serverCode)
	if err != nil {
		return err
	}
	if s.Status != StatusEnabled {
		return ErrServerDisabled
	}
	return nil
}

// sessionFor 取该服务的活跃会话；缺失时建立（解密 Token -> 握手 -> 入池）
func (uc *Usecase) sessionFor(ctx context.Context, s *Server) (*Client, error) {
	if c := uc.manager.get(s.Code); c != nil {
		return c, nil
	}
	token, err := uc.crypto.Decrypt(s.TokenEnc)
	if err != nil {
		return nil, err
	}
	cli := NewClient(ClientConfig{Transport: s.Transport, BaseURL: s.BaseURL, Headers: s.Headers, Token: token})
	hctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, _, err := cli.Handshake(hctx); err != nil {
		cli.Close()
		return nil, err
	}
	uc.manager.put(s.Code, cli)
	return cli, nil
}
