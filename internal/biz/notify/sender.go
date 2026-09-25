package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	sendTimeout          = 10 * time.Second
	emailPortImplicitTLS = 465 // 465 隐式 TLS，其余端口尝试 STARTTLS
	errBodyMaxBytes      = 256 // Webhook 错误响应体读取上限
)

// webhookClient Webhook 投递客户端（无连接池复用诉求，全局共享即可）
var webhookClient = &http.Client{Timeout: sendTimeout}

// sendEmail SMTP 邮件投递：465 隐式 TLS / 其余端口 STARTTLS（服务端支持时），
// 全程设置 Socket Deadline 兜底超时；正文 base64 编码保证中文兼容。
func (uc *Usecase) sendEmail(ctx context.Context, ch *Channel, title, content string) error {
	password, err := uc.crypto.Decrypt(ch.SMTPPassEnc)
	if err != nil {
		return fmt.Errorf("decrypt smtp password: %w", err)
	}
	addr := fmt.Sprintf("%s:%d", ch.SMTPHost, ch.SMTPPort)

	d := net.Dialer{Timeout: sendTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > sendTimeout {
		_ = conn.SetDeadline(time.Now().Add(sendTimeout))
	}

	var c *smtp.Client
	if ch.SMTPPort == emailPortImplicitTLS {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: ch.SMTPHost})
		if err := tlsConn.Handshake(); err != nil {
			_ = conn.Close()
			return fmt.Errorf("tls handshake: %w", err)
		}
		c, err = smtp.NewClient(tlsConn, ch.SMTPHost)
	} else {
		c, err = smtp.NewClient(conn, ch.SMTPHost)
	}
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	if ch.SMTPPort != emailPortImplicitTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: ch.SMTPHost}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	if ch.SMTPUser != "" {
		if err := c.Auth(smtp.PlainAuth("", ch.SMTPUser, password, ch.SMTPHost)); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	msg := buildMail(ch.SMTPFrom, ch.Recipients, title, content)
	if err := c.Mail(ch.SMTPFrom); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range ch.Recipients {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close body: %w", err)
	}
	return c.Quit()
}

// buildMail 组装 UTF-8 邮件（标题 Q 编码、正文 base64，每行 76 字符）
func buildMail(from string, to []string, title, content string) []byte {
	var b bytes.Buffer
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", title) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")

	body := base64.StdEncoding.EncodeToString([]byte(title + "\n\n" + content))
	for len(body) > 76 {
		b.WriteString(body[:76] + "\r\n")
		body = body[76:]
	}
	b.WriteString(body + "\r\n")
	return b.Bytes()
}

// webhookPayload Webhook 请求体
type webhookPayload struct {
	Source    string `json:"source"`
	RuleName  string `json:"rule_name"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// sendWebhook Webhook 投递：POST JSON + HMAC-SHA256 签名头（风格对齐商户开放 API）。
// 验签串 = timestamp + "." + 原始请求体，签名 = hex(HMAC-SHA256(secret, 验签串))；
// 接收方校验 X-Timestamp 偏差（建议 ±300s）与 X-Sign 一致性。网络错误/5xx 重试 1 次。
func (uc *Usecase) sendWebhook(ctx context.Context, ch *Channel, source Source, title, content, ruleName string) error {
	secret, err := uc.crypto.Decrypt(ch.WebhookEnc)
	if err != nil {
		return fmt.Errorf("decrypt webhook secret: %w", err)
	}
	body, err := json.Marshal(webhookPayload{
		Source: string(source), RuleName: ruleName, Title: title, Content: content, Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Second) // 重试前稍等，避免瞬时抖动
		}
		lastErr = uc.postWebhook(ctx, ch.WebhookURL, secret, body)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (uc *Usecase) postWebhook(ctx context.Context, url, secret string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Sign", hex.EncodeToString(mac.Sum(nil)))

	resp, err := webhookClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, errBodyMaxBytes))
	return fmt.Errorf("webhook responded %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
}

// ---- IM 群机器人（企业微信 / 钉钉 / 飞书）----
// 三平台的机器人 webhook 均 POST JSON 且以业务码判定成败（HTTP 均 200），
// 统一 postRobot：非 2xx 或响应体业务码非 0 视为失败，返回平台错误文案。

// robotResp 机器人响应体（三平台字段名一致的公因子：errcode/code/StatusCode）
type robotResp struct {
	Errcode     int    `json:"errcode"`
	Errmsg      string `json:"errmsg"`
	Code        int    `json:"code"`
	Msg         string `json:"msg"`
	StatusCode  int    `json:"StatusCode"`
	StatusMsg   string `json:"StatusMessage"`
	Description string `json:"description"`
}

func (r robotResp) notOK() (string, bool) {
	switch {
	case r.Errcode != 0:
		return fmt.Sprintf("errcode=%d %s", r.Errcode, r.Errmsg), true
	case r.Code != 0:
		return fmt.Sprintf("code=%d %s", r.Code, pickMsg(r.Msg, r.Description)), true
	case r.StatusCode != 0:
		return fmt.Sprintf("status=%d %s", r.StatusCode, r.StatusMsg), true
	}
	return "", false
}

func pickMsg(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// postRobot POST JSON 到机器人地址并校验业务码
func (uc *Usecase) postRobot(ctx context.Context, url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := webhookClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, errBodyMaxBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("robot responded %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var rr robotResp
	if jerr := json.Unmarshal(snippet, &rr); jerr == nil {
		if msg, bad := rr.notOK(); bad {
			return fmt.Errorf("robot rejected: %s", msg)
		}
	}
	return nil
}

// robotSecret 解密渠道密钥（未配置时为空串，AESGCM 空串直通）
func (uc *Usecase) robotSecret(ch *Channel) (string, error) {
	secret, err := uc.crypto.Decrypt(ch.WebhookEnc)
	if err != nil {
		return "", fmt.Errorf("decrypt robot secret: %w", err)
	}
	return secret, nil
}

// sendWecom 企业微信群机器人：markdown 消息（key 已含在 webhook URL 中，无需密钥）
func (uc *Usecase) sendWecom(ctx context.Context, ch *Channel, title, content string) error {
	return uc.postRobot(ctx, ch.WebhookURL, map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"content": "**" + title + "**\n" + content},
	})
}

// sendDingtalk 钉钉群机器人：markdown 消息；配置了密钥时按官方加签规则
// 在 URL 追加 &timestamp=<毫秒>&sign=<urlEscape(base64(HMAC-SHA256(secret, ts+"\n"+secret)))>
func (uc *Usecase) sendDingtalk(ctx context.Context, ch *Channel, title, content string) error {
	hookURL := ch.WebhookURL
	if secret, err := uc.robotSecret(ch); err != nil {
		return err
	} else if secret != "" {
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(ts + "\n" + secret))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		sep := "?"
		if strings.Contains(hookURL, "?") {
			sep = "&" // 官方地址自带 ?access_token=…，无参地址用 ?
		}
		hookURL += sep + "timestamp=" + ts + "&sign=" + sign
	}
	return uc.postRobot(ctx, hookURL, map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"title": title, "text": "**" + title + "**\n\n" + content},
	})
}

// sendFeishu 飞书群机器人：text 消息；配置了密钥时按官方加签规则在请求体
// 附 timestamp（秒，字符串）与 sign=base64(HMAC-SHA256(secret, ts+"\n"+secret))
func (uc *Usecase) sendFeishu(ctx context.Context, ch *Channel, title, content string) error {
	payload := map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": title + "\n" + content},
	}
	if secret, err := uc.robotSecret(ch); err != nil {
		return err
	} else if secret != "" {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(ts + "\n" + secret))
		payload["timestamp"] = ts
		payload["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	return uc.postRobot(ctx, ch.WebhookURL, payload)
}
