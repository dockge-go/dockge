package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	v1 "dockge/app/dockge/api/v1"
)

type fakeSetupChecker struct{ need bool }

func (f fakeSetupChecker) CheckNeedSetup(ctx context.Context) (bool, error) { return f.need, nil }

func setupRouter(need bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SetupRequired(fakeSetupChecker{need: need}))
	r.GET("/v1/stacks", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/v1/setup/need", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/v1/setup", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/v1/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/v1/robots.txt", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/v1/auto-login", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestSetupRequiredBlocksWhenNeeded(t *testing.T) {
	r := setupRouter(true)
	req := httptest.NewRequest(http.MethodGet, "/v1/stacks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("GET /v1/stacks when setup needed = %d, want 403", w.Code)
	}
}

func TestSetupRequiredAllowlistWhenNeeded(t *testing.T) {
	r := setupRouter(true)
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/v1/setup/need"},
		{http.MethodPost, "/v1/setup"},
		{http.MethodGet, "/v1/health"},
		{http.MethodGet, "/v1/robots.txt"},
		{http.MethodPost, "/v1/auto-login"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s %s when setup needed = %d, want 200", tc.method, tc.path, w.Code)
		}
	}
}

func TestSetupRequiredPassesWhenNotNeeded(t *testing.T) {
	r := setupRouter(false)
	req := httptest.NewRequest(http.MethodGet, "/v1/stacks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /v1/stacks when setup done = %d, want 200", w.Code)
	}
}

// fakeProxyLogin implements the proxyLoginService interface for testing.
type fakeProxyLogin struct{}

func (f fakeProxyLogin) ProxyLogin(ctx context.Context, username string, remoteAddr string) (*v1.LoginResponseData, error) {
	return &v1.LoginResponseData{
		AccessToken: "test-jwt-token",
		User:        v1.MeUserData{ID: 1, Username: "testuser"},
	}, nil
}

func TestSetupRequiredProtectsWhenNeeded(t *testing.T) {
	r := setupRouter(true)
	req := httptest.NewRequest(http.MethodGet, "/v1/stacks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("GET /v1/stacks when setup needed = %d, want 403", w.Code)
	}
}

// TestProxyAuthBeforeStrictAuth verifies that in proxy mode, ProxyAuth
// executes before StrictAuth, allowing requests with a valid
// X-Forwarded-User header to reach protected routes. In jwt mode, missing
// auth still results in 401.
func TestProxyAuthBeforeStrictAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// ---- proxy mode: ProxyAuth should run first, then StrictAuth ----
	r := gin.New()

	// Register middleware in the order that http.go now uses for proxy mode:
	// ProxyAuth first, then StrictAuth
	conf := viper.New()
	conf.Set("security.auth.mode", "proxy")
	conf.Set("security.auth.proxy.username_header", "X-Forwarded-User")
	r.Use(ProxyAuth(fakeProxyLogin{}, nil, conf))
	r.Use(StrictAuth(nil, nil))

	// Protected route
	r.GET("/v1/me", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test: request with X-Forwarded-User header should succeed in proxy mode
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("X-Forwarded-User", "testuser")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("proxy mode: GET /v1/me with X-Forwarded-User = %d, want 200", w.Code)
	}

	// Test: request without X-Forwarded-User header should fail in proxy mode
	req2 := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("proxy mode: GET /v1/me without header = %d, want 401", w2.Code)
	}

	// ---- jwt mode: StrictAuth should still reject missing auth ----
	gin.SetMode(gin.TestMode)
	r2 := gin.New()
	// In jwt mode, ProxyAuth is a no-op (pass-through), StrictAuth runs
	conf2 := viper.New()
	conf2.Set("security.auth.mode", "jwt")
	r2.Use(ProxyAuth(fakeProxyLogin{}, nil, conf2))
	r2.Use(StrictAuth(nil, nil))

	r2.GET("/v1/me", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test: request without auth in jwt mode should fail
	req3 := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	w3 := httptest.NewRecorder()
	r2.ServeHTTP(w3, req3)
	if w3.Code != http.StatusUnauthorized {
		t.Errorf("jwt mode: GET /v1/me without auth = %d, want 401", w3.Code)
	}
}
