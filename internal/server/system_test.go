package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemRequiresAuthentication(t *testing.T) {
	engine := newTestServer(t)
	if code, _ := call(t, engine, "GET", "/v1/system", "", nil); code != http.StatusForbidden {
		t.Fatalf("uninitialized system=%d", code)
	}
	code, payload := call(t, engine, "POST", "/v1/setup", "", map[string]string{"username": "admin", "password": "s3cret-pass"})
	if code != 200 {
		t.Fatalf("setup %d %v", code, payload)
	}
	code, payload = call(t, engine, "POST", "/v1/login", "", map[string]string{"username": "admin", "password": "s3cret-pass"})
	if code != 200 {
		t.Fatalf("login %d %v", code, payload)
	}
	token := dataString(payload, "accessToken")
	if token == "" {
		t.Fatal("missing token")
	}
	if code, _ := call(t, engine, "POST", "/v1/events/refresh", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("unauthorized events refresh=%d", code)
	}
	code, payload = call(t, engine, "POST", "/v1/events/refresh", token, nil)
	if code != http.StatusOK {
		t.Fatalf("events refresh %d %v", code, payload)
	}
	if revision, ok := payload["data"].(map[string]any)["revision"].(float64); !ok || revision <= 0 {
		t.Fatalf("missing refresh boundary: %v", payload)
	}
	if code, _ := call(t, engine, "GET", "/v1/system", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("unauthorized system=%d", code)
	}
	code, payload = call(t, engine, "GET", "/v1/system", token, nil)
	if code != 200 {
		t.Fatalf("system %d %v", code, payload)
	}
	data := payload["data"].(map[string]any)
	if data["sampledAt"] == "" || data["errors"] == nil {
		t.Fatalf("missing sampling provenance: %v", data)
	}
	if data["cpuPercent"] != nil {
		t.Fatal("first CPU sample must be unknown")
	}
}

// 实时采样与认证响应不能被浏览器或代理存成历史状态。
func TestAPIResponsesDoNotStore(t *testing.T) {
	engine := newTestServer(t)
	for _, path := range []string{"/v1/health", "/v1/system", "/v1/resources", "/v1/stacks", "/v1/events/refresh", "/v1/not-found"} {
		t.Run(path, func(t *testing.T) {
			method := http.MethodGet
			if path == "/v1/events/refresh" {
				method = http.MethodPost
			}
			result := httptest.NewRecorder()
			engine.ServeHTTP(result, httptest.NewRequest(method, path, nil))
			if got := result.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("%s Cache-Control=%q", path, got)
			}
		})
	}
}
