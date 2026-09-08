// Package jwt 提供 JWT 签发与解析能力，支持 shake256 密码绑定（改密后旧 token 自动失效）。
package jwt

import (
	"errors"
	"strings"
	"time"

	"dockge/pkg/hash"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// Claims 是 JWT payload：携带用户 ID 与 shake256 密码摘要。
// 改密后 h 值变化，旧 token 校验失败。
type Claims struct {
	UserId uint   `json:"uid"`
	H      string `json:"h"` // shake256(password)
	jwt.RegisteredClaims
}

// JWT 是 HS256 签名的 JWT 工具。
type JWT struct {
	key []byte
}

var Package = do.Package(do.Lazy(New))

func New(i do.Injector) (*JWT, error) {
	conf := do.MustInvoke[*viper.Viper](i)
	return &JWT{key: []byte(conf.GetString("security.jwt.key"))}, nil
}

// GenToken 签发 JWT，携带 shake256 密码绑定。
func (j *JWT) GenToken(userId uint, passwordHash string, expiresAt time.Time) (string, error) {
	h := hash.Shake256(passwordHash)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserId: userId,
		H:      h,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	})
	s, err := token.SignedString(j.key)
	if err != nil {
		return "", err
	}
	return s, nil
}

// ParseToken 解析并校验 JWT。
func (j *JWT) ParseToken(tokenString string) (*Claims, error) {
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	if strings.TrimSpace(tokenString) == "" {
		return nil, errors.New("token is empty")
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return j.key, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, err
}
