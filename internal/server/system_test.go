package server

import (
	"net/http"
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
