package data

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/biz/auth"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"go.uber.org/zap"
)

// 令牌类型声明：access 与 refresh 结构一致但 typ 不同，
// 防止 refresh token 被直接当作 access token 调用受保护接口。
// app-access/app-refresh 为应用用户体系专用，与后台账号 token 相互隔离（typ 不匹配即拒绝）。
const (
	tokenTypeAccess     = "access"
	tokenTypeRefresh    = "refresh"
	tokenTypeAppAccess  = "app-access"
	tokenTypeAppRefresh = "app-refresh"
)

type claims struct {
	UserID    uint   `json:"uid"`
	Username  string `json:"username"`
	SessionID string `json:"sid"` // 会话 ID：token 仅在对应会话存活期间有效
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type jwtIssuer struct {
	secret       []byte
	issuer       string
	expireHours  int
	refreshHours int
}

// NewJWTIssuer JWT 令牌签发实现（后台账号体系）
func NewJWTIssuer(c *conf.Bootstrap) auth.TokenIssuer {
	return newJWTIssuer(c)
}

// NewAppTokenIssuer 应用用户令牌签发实现（复用同一 secret/issuer/过期配置，typ 隔离）
func NewAppTokenIssuer(c *conf.Bootstrap) appuser.TokenIssuer {
	return newJWTIssuer(c)
}

func newJWTIssuer(c *conf.Bootstrap) *jwtIssuer {
	// 弱密钥告警：默认值或过短的 secret 可被离线爆破伪造令牌
	if len(c.JWT.Secret) < 32 {
		logger.Warn("jwt secret 长度不足 32 位，存在被爆破风险，请尽快修改 configs/config.yaml",
			zap.Int("length", len(c.JWT.Secret)))
	}
	return &jwtIssuer{
		secret:       []byte(c.JWT.Secret),
		issuer:       c.JWT.Issuer,
		expireHours:  c.JWT.ExpireHours,
		refreshHours: c.JWT.RefreshHours,
	}
}

func (j *jwtIssuer) newClaims(s auth.Subject, typ string, ttl time.Duration) *claims {
	now := time.Now()
	return &claims{
		UserID:    s.UserID,
		Username:  s.Username,
		SessionID: s.SessionID,
		TokenType: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   s.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
}

func (j *jwtIssuer) sign(c *claims) (string, time.Time, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	token, err := t.SignedString(j.secret)
	return token, c.ExpiresAt.Time, err
}

func (j *jwtIssuer) IssueAccessToken(s auth.Subject) (string, time.Time, error) {
	return j.sign(j.newClaims(s, tokenTypeAccess, time.Duration(j.expireHours)*time.Hour))
}

func (j *jwtIssuer) IssueRefreshToken(s auth.Subject) (string, error) {
	token, _, err := j.sign(j.newClaims(s, tokenTypeRefresh, time.Duration(j.refreshHours)*time.Hour))
	return token, err
}

func (j *jwtIssuer) parse(token, wantType string) (*auth.Subject, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if c.TokenType != wantType {
		return nil, errors.New("token type mismatch")
	}
	return &auth.Subject{UserID: c.UserID, Username: c.Username, SessionID: c.SessionID}, nil
}

func (j *jwtIssuer) ParseAccessToken(token string) (*auth.Subject, error) {
	return j.parse(token, tokenTypeAccess)
}
func (j *jwtIssuer) ParseRefreshToken(token string) (*auth.Subject, error) {
	return j.parse(token, tokenTypeRefresh)
}

// ---- 应用用户令牌（typ 为 app-access/app-refresh，sid 留空：应用用户无服务端会话状态） ----

func (j *jwtIssuer) IssueAppAccessToken(uid uint, username string) (string, time.Time, error) {
	return j.sign(j.newClaims(auth.Subject{UserID: uid, Username: username}, tokenTypeAppAccess, time.Duration(j.expireHours)*time.Hour))
}

func (j *jwtIssuer) IssueAppRefreshToken(uid uint, username string) (string, error) {
	token, _, err := j.sign(j.newClaims(auth.Subject{UserID: uid, Username: username}, tokenTypeAppRefresh, time.Duration(j.refreshHours)*time.Hour))
	return token, err
}

// parseApp 解析应用用户令牌并校验 typ（与后台 token 隔离，混用即拒绝）
func (j *jwtIssuer) parseApp(token, wantType string) (uint, string, error) {
	s, err := j.parse(token, wantType)
	if err != nil {
		return 0, "", err
	}
	return s.UserID, s.Username, nil
}

func (j *jwtIssuer) ParseAppAccessToken(token string) (uint, string, error) {
	return j.parseApp(token, tokenTypeAppAccess)
}
func (j *jwtIssuer) ParseAppRefreshToken(token string) (uint, string, error) {
	return j.parseApp(token, tokenTypeAppRefresh)
}
