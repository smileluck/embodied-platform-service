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
