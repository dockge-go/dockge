package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"

	"dockge/pkg/jwt"
	"dockge/pkg/log"
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

// fakeSessions 实现 sessionChecker：err 非 nil 时模拟「账号已停用/改密」。
type fakeSessions struct{ err error }

func (f fakeSessions) CheckSession(ctx context.Context, uid uint, h string) error { return f.err }

// TestStrictAuthChecksSession 逐请求会话校验（D12 接线）：
// token 有效但会话校验失败（停用/删除/改密）→ 401；校验通过 → 放行。
func TestStrictAuthChecksSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 构造真 JWT（viper 提供 security.jwt.key）
	injector := do.New()
	conf := viper.New()
	conf.Set("security.jwt.key", "unit-test-key")
	do.ProvideValue(injector, conf)
	do.Provide(injector, jwt.New)
	j := do.MustInvoke[*jwt.JWT](injector)
	token, err := j.GenToken(7, "hash-value", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	newRouter := func(sessions sessionChecker) *gin.Engine {
		r := gin.New()
		r.Use(StrictAuth(j, &log.Logger{Logger: zerolog.Nop()}, sessions))
		r.GET("/v1/me", func(c *gin.Context) { c.Status(http.StatusOK) })
		return r
	}

	// 会话校验失败（账号被停用）→ 401
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	newRouter(fakeSessions{err: errors.New("deactivated")}).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("deactivated session = %d, want 401", w.Code)
	}

	// 会话校验通过 → 200
	req2 := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	newRouter(fakeSessions{}).ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("valid session = %d, want 200", w2.Code)
	}

	// sessions 为 nil（未注入）时保持旧行为放行——向后兼容
	req3 := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	newRouter(nil).ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("nil session checker = %d, want 200", w3.Code)
	}
}
