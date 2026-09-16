// Package platform embodied-platform 平台侧 HTTP 客户端集合（基础设施层）。
//
// 本系统作为平台的业务接入方，持有三类凭证（全部来自 config 的 platform 段）：
//   - 商户 AppKey/AppSecret：开放面 /open-api/v1（设备域，HMAC 签名，经平台 SDK）
//   - 管理面服务账号：/api/v1（租户同步 / 型号管理 / 平台用户列表，JWT）
//   - storage-gateway API Key：/openapi/storage/v1（文件存储）
//
// 另有 IdentityClient：用「用户本人平台 token」做身份自省（profile）与自身数据代理，
// 对应平台统一账号决策「token 双用 + profile 当 introspection」。
package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Error 平台返回的非 0 业务错误（携带 HTTP 状态码与信封 msg），供上层映射为本地响应
type Error struct {
	HTTPStatus int
	Code       int
	Msg        string
}

func (e *Error) Error() string {
	return fmt.Sprintf("platform: http %d code %d: %s", e.HTTPStatus, e.Code, e.Msg)
}

// envelope 平台统一响应信封 {code,msg,data}
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// Page 平台列表分页信息
type Page struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// listData 平台列表响应 data 形态 {list,page}
type listData struct {
	List json.RawMessage `json:"list"`
	Page Page            `json:"page"`
}

// do 通用请求执行：解包统一信封，code!=0 返回 *Error。
// bearer 非空时附加 Authorization 头；body 非 nil 时 JSON 序列化；out 非 nil 时解包 data。
func do(ctx context.Context, hc *http.Client, method, endpoint, bearer string, body, out any) error {
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("platform: marshal body: %w", err)
		}
		raw = b
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("platform: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("platform: do request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("platform: read response: %w", err)
	}
	var env envelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return &Error{HTTPStatus: resp.StatusCode, Code: -1, Msg: string(respBody)}
	}
	if env.Code != 0 {
		return &Error{HTTPStatus: resp.StatusCode, Code: env.Code, Msg: env.Msg}
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("platform: unmarshal data: %w", err)
		}
	}
	return nil
}

// doRaw 同 do，但把信封 data 原文返回（调用方自行解包）
func doRaw(ctx context.Context, hc *http.Client, method, endpoint, bearer string, body any, out *[]byte) error {
	var data json.RawMessage
	if err := do(ctx, hc, method, endpoint, bearer, body, &data); err != nil {
		return err
	}
	*out = data
	return nil
}

// doList 列表请求：解包 {list,page}
func doList(ctx context.Context, hc *http.Client, endpoint, bearer string, listOut any) (*Page, error) {
	var raw []byte
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("platform: build request: %w", err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("platform: do request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("platform: read response: %w", err)
	}
	var env struct {
		Code int      `json:"code"`
		Msg  string   `json:"msg"`
		Data listData `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, &Error{HTTPStatus: resp.StatusCode, Code: -1, Msg: string(respBody)}
	}
	if env.Code != 0 {
		return nil, &Error{HTTPStatus: resp.StatusCode, Code: env.Code, Msg: env.Msg}
	}
	if len(env.Data.List) > 0 {
		if err := json.Unmarshal(env.Data.List, listOut); err != nil {
			return nil, fmt.Errorf("platform: unmarshal list: %w", err)
		}
	}
	return &env.Data.Page, nil
}
