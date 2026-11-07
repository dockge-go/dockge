package server

import (
	"net/http"
	"strings"
	"testing"
)

// TestOutsideStackAPIsRejectWithDetails 防止管理范围错误变成 500 或丢失具体栈名、路径。
func TestOutsideStackAPIsRejectWithDetails(t *testing.T) {
	engine := newTestServer(t)
	code, payload := call(t, engine, "POST", "/v1/setup", "", map[string]string{"username": "admin", "password": "s3cret-pass"})
	if code != http.StatusOK {
		t.Fatalf("setup: %d %v", code, payload)
	}
	token := dataString(payload, "accessToken")
	for _, endpoint := range []struct{ method, suffix string }{
		{"GET", ""}, {"PUT", ""}, {"GET", "/stats"}, {"GET", "/delete-plan"}, {"DELETE", ""}, {"POST", "/services/web/stop"},
	} {
		t.Run(endpoint.method+endpoint.suffix, func(t *testing.T) {
			var body any
			if endpoint.method == "PUT" {
				body = map[string]string{"name": "external", "yaml": "services: {}\n", "env": ""}
			}
			code, payload := call(t, engine, endpoint.method, "/v1/stacks/external"+endpoint.suffix, token, body)
			message, _ := payload["message"].(string)
			if code != http.StatusNotFound || !strings.Contains(message, "external") || !strings.Contains(message, "stacks") {
				t.Fatalf("管理范围外应返回带栈名与路径的 404: %d %v", code, payload)
			}
		})
	}
}
