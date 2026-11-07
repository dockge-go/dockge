package server

import (
	"net/http"
	"testing"
)

// TestLoginRateLimitBelongsToServer 保证重建服务时不继承其他实例的登录计数，且不削弱单实例限流。
func TestLoginRateLimitBelongsToServer(t *testing.T) {
	first := newTestServer(t)
	if code, payload := call(t, first, "POST", "/v1/setup", "", map[string]string{"username": "admin", "password": "s3cret-pass"}); code != http.StatusOK {
		t.Fatalf("首个实例初始化=%d %v", code, payload)
	}
	for n := 0; n < 10; n++ {
		code, payload := call(t, first, "POST", "/v1/login", "", map[string]string{})
		if code != http.StatusBadRequest {
			t.Fatalf("第 %d 次登录参数校验=%d %v", n+1, code, payload)
		}
	}
	if code, payload := call(t, first, "POST", "/v1/login", "", map[string]string{}); code != http.StatusTooManyRequests {
		t.Fatalf("同实例超限=%d %v，期望 429", code, payload)
	}

	second := newTestServer(t)
	if code, payload := call(t, second, "POST", "/v1/setup", "", map[string]string{"username": "admin", "password": "s3cret-pass"}); code != http.StatusOK {
		t.Fatalf("新实例初始化=%d %v", code, payload)
	}
	if code, payload := call(t, second, "POST", "/v1/login", "", map[string]string{}); code != http.StatusBadRequest {
		t.Fatalf("新实例继承旧登录计数=%d %v，期望 400", code, payload)
	}
	if code, payload := call(t, first, "POST", "/v1/login", "", map[string]string{}); code != http.StatusTooManyRequests {
		t.Fatalf("新建实例不应重置旧实例限流=%d %v，期望 429", code, payload)
	}
}
