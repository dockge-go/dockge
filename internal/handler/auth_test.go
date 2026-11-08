package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "dockge/api/v1"
	"dockge/internal/service"
	"github.com/gin-gonic/gin"
)

type autoLoginStub struct {
	service.AuthService
	err error
}

func (s autoLoginStub) AutoLogin(context.Context) (*v1.LoginResponseData, error) { return nil, s.err }

// TestAutoLoginErrorStatus 防止存储/签发失败被误报成登录失效。
func TestAutoLoginErrorStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		detail string
	}{
		{"登录失效", v1.ErrUnauthorized, 401, "登录失效"},
		{"业务禁止", &v1.Error{Code: 403, Message: "OIDC 禁止免登录"}, 403, "OIDC"},
		{"存储错误", errors.New("read account: database is closed"), 500, "database is closed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := gin.New()
			h := &AuthHandler{AuthService: autoLoginStub{err: tc.err}}
			e.POST("/auto-login", h.AutoLogin)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auto-login", nil))
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.detail) {
				t.Fatalf("响应=%d %s，期望 %d %s", w.Code, w.Body.String(), tc.status, tc.detail)
			}
		})
	}
}
