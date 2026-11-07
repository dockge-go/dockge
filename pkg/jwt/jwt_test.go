package jwt

import (
	"testing"
	"time"

	"dockge/pkg/hash"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

// TestTokenContract 解析只能接受本服务签发的 HS256、带有效期的会话。
func TestTokenContract(t *testing.T) {
	cfg := viper.New()
	cfg.Set("security.jwt.key", "test-session-key")
	j := NewJWT(cfg)
	expiry := time.Now().Add(time.Hour).Truncate(time.Second)
	raw, err := j.GenToken(1, "password-hash", expiry)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := j.ParseToken(raw)
	if err != nil || claims.UserId != 1 || claims.H != hash.Shake256("password-hash") || !claims.ExpiresAt.Time.Equal(expiry) {
		t.Fatalf("签发与解析不一致：%v %v", claims, err)
	}
	for _, tc := range []struct {
		name   string
		method jwtlib.SigningMethod
		expiry *jwtlib.NumericDate
	}{
		{"HS384", jwtlib.SigningMethodHS384, jwtlib.NewNumericDate(expiry)},
		{"HS512", jwtlib.SigningMethodHS512, jwtlib.NewNumericDate(expiry)},
		{"缺少过期时间", jwtlib.SigningMethodHS256, nil},
		{"已过期", jwtlib.SigningMethodHS256, jwtlib.NewNumericDate(time.Now().Add(-time.Hour))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := jwtlib.NewWithClaims(tc.method, Claims{UserId: 1, H: hash.Shake256("password-hash"), RegisteredClaims: jwtlib.RegisteredClaims{ExpiresAt: tc.expiry}})
			signed, err := token.SignedString(j.key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := j.ParseToken(signed); err == nil {
				t.Fatalf("错误接受 %s 会话", tc.name)
			}
		})
	}
	if _, err := j.ParseToken(""); err == nil {
		t.Fatal("错误接受空会话")
	}
}
