package service

import (
	"context"
	"strings"
	"testing"

	v1 "dockge/api/v1"
	"dockge/pkg/config"
	"dockge/pkg/hash"
	"dockge/pkg/jwt"
	"dockge/pkg/rate"

	"github.com/spf13/viper"
)

func newTestAuthService(t *testing.T) (*authService, uint) {
	t.Helper()
	repo, id := newTestAuthRepo(t)
	v := viper.New()
	v.Set("security.jwt.key", "test-session-key")
	return &authService{repo: repo, jwt: jwt.NewJWT(v), loginLimiter: rate.NewKeyed(loginRateLimit, loginRateWindow, loginMaxKeys)}, id
}

func TestValidateUsername(t *testing.T) {
	for _, c := range []struct {
		in   string
		want bool
	}{{"ab", true}, {"a.b_c-d", true}, {"user123", true}, {"a", false}, {strings.Repeat("a", 33), false}, {"a b", false}, {"用户名", false}, {"a:b", false}, {"", false}, {strings.Repeat("a", 32), true}} {
		if got := ValidateUsername(c.in); got != c.want {
			t.Errorf("ValidateUsername(%q)=%v want %v", c.in, got, c.want)
		}
	}
}

func TestLoginNoEnumeration(t *testing.T) {
	s, id := newTestAuthService(t)
	ctx := context.Background()
	if err := s.repo.UpdatePassword(ctx, id, hashBcryptForTest("RealPass1")); err != nil {
		t.Fatal(err)
	}
	var message string
	for _, name := range []string{"ghost", "admin"} {
		_, err := s.Login(ctx, &v1.LoginRequest{Username: name, Password: "WrongPass1"}, "127.0.0.1")
		e, ok := err.(*v1.Error)
		if !ok || e.Code != 401 {
			t.Fatalf("login=%v", err)
		}
		if message != "" && message != e.Message {
			t.Fatal("account enumeration")
		}
		message = e.Message
	}
}

func TestSingleUserSessionAndPassword(t *testing.T) {
	s, id := newTestAuthService(t)
	ctx := context.Background()
	if err := s.repo.UpdatePassword(ctx, id, hashBcryptForTest("RealPass1")); err != nil {
		t.Fatal(err)
	}
	data, err := s.Login(ctx, &v1.LoginRequest{Username: "admin", Password: "RealPass1"}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := s.jwt.ParseToken(data.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if data.ExpiresAt != claims.ExpiresAt.Time.UnixMilli() {
		t.Fatal("登录响应有效期与 JWT 不一致")
	}
	if err := s.CheckSession(ctx, id, claims.H); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckSession(ctx, id+1, claims.H); err == nil {
		t.Fatal("another uid accepted")
	}
	if _, err := s.Me(ctx, id+1); err == nil {
		t.Fatal("another uid accessed me")
	}
	if _, err := s.Setup(ctx, &v1.SetupRequest{Username: "other", Password: "NewPass1"}); err == nil {
		t.Fatal("second setup accepted")
	}
	if err := s.ChangePassword(ctx, id, &v1.ChangePasswordRequest{OldPassword: "RealPass1", NewPassword: "NewPass1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckSession(ctx, id, claims.H); err == nil {
		t.Fatal("old session accepted after password change")
	}
}

func TestOIDCDisablesLocalBypass(t *testing.T) {
	s, id := newTestAuthService(t)
	ctx := context.Background()
	s.oidc = config.OIDC{Enabled: true, Issuer: "https://id.example", ClientID: "dockge", Subject: "one"}
	_ = s.repo.SetSetting(ctx, "disableAuth", "true", "security")
	for _, err := range []error{
		func() error {
			_, e := s.Login(ctx, &v1.LoginRequest{Username: "admin", Password: "RealPass1"}, "127.0.0.1")
			return e
		}(),
		func() error {
			_, e := s.Setup(ctx, &v1.SetupRequest{Username: "admin", Password: "RealPass1"})
			return e
		}(),
		s.ChangePassword(ctx, id, &v1.ChangePasswordRequest{OldPassword: "x", NewPassword: "RealPass1"}),
		s.ToggleDisableAuth(ctx, id, true, "x"),
		func() error { _, e := s.AutoLogin(ctx); return e }(),
	} {
		if err == nil {
			t.Fatal("local bypass accepted in OIDC mode")
		}
	}
	if need, err := s.CheckNeedSetup(ctx); err != nil || need {
		t.Fatal("OIDC should not need local setup")
	}
	u, _ := s.repo.SingleUser(ctx)
	if err := s.CheckSession(ctx, id, hash.Shake256(u.Password)); err == nil {
		t.Fatal("old local JWT accepted in OIDC mode")
	}
}
