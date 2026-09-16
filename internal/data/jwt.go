package data

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"go.uber.org/zap"
)

// 令牌类型声明：管理端登录已委外给平台（token 双用，本系统不再签发），
// 本文件仅服务应用用户体系（app-access / app-refresh，typ 与任何平台令牌隔离）。
const (
	tokenTypeAppAccess  = "app-access"
	tokenTypeAppRefresh = "app-refresh"
)

type claims struct {
	UserID    uint   `json:"uid"`
	Username  string `json:"username"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type jwtIssuer struct {
	secret       []byte
	issuer       string
	expireHours  int
	refreshHours int
}

// NewAppTokenIssuer 应用用户令牌签发实现
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

func (j *jwtIssuer) newClaims(uid uint, username, typ string, ttl time.Duration) *claims {
	now := time.Now()
	return &claims{
		UserID:    uid,
		Username:  username,
		TokenType: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   username,
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

func (j *jwtIssuer) IssueAppAccessToken(uid uint, username string) (string, time.Time, error) {
	return j.sign(j.newClaims(uid, username, tokenTypeAppAccess, time.Duration(j.expireHours)*time.Hour))
}

func (j *jwtIssuer) IssueAppRefreshToken(uid uint, username string) (string, error) {
	token, _, err := j.sign(j.newClaims(uid, username, tokenTypeAppRefresh, time.Duration(j.refreshHours)*time.Hour))
	return token, err
}

// parseApp 解析应用用户令牌并校验 typ
func (j *jwtIssuer) parseApp(token, wantType string) (uint, string, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return j.secret, nil
	})
	if err != nil {
		return 0, "", err
	}
	if c.TokenType != wantType {
		return 0, "", errors.New("token type mismatch")
	}
	return c.UserID, c.Username, nil
}

func (j *jwtIssuer) ParseAppAccessToken(token string) (uint, string, error) {
	return j.parseApp(token, tokenTypeAppAccess)
}

func (j *jwtIssuer) ParseAppRefreshToken(token string) (uint, string, error) {
	return j.parseApp(token, tokenTypeAppRefresh)
}
