package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockge/internal/handler"
	"dockge/internal/repository"
	"dockge/internal/service"
	"dockge/pkg/config"
	"dockge/pkg/jwt"
	"dockge/pkg/log"
	httpx "dockge/pkg/server/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// 本文件是 HTTP 接口的端到端契约测试：用完整 DI 容器 + temp 存储 +
// 桩容器 CLI（不依赖本机 docker 与网络），覆盖鉴权、栈 CRUD/生命周期、
// 设置、composerize 与已移除端点的 404 契约。

// stubCLI 写一个假的容器 CLI：compose/stats/network 调用返回可预期的输出，
// 使栈相关接口无需真实 docker 也能断言行为。
func stubCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-stub")
	script := `#!/bin/sh
case "$*" in
  "compose ls --all --format json") echo '[]' ;;
  "network ls --format {{.Name}}") echo "bridge host none" ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// newTestServer 装配与生产同构的服务（去掉 app.App 生命周期），返回 gin 引擎。
func newTestServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	stub := stubCLI(t)
	t.Setenv("DOCKGE_SECURITY_JWT_KEY", "test-key")
	t.Setenv("DOCKGE_STACKS_DIR", filepath.Join(root, "stacks"))
	t.Setenv("DOCKGE_DATA_DB_USER_DSN", filepath.Join(root, "dockge.db"))
	t.Setenv("DOCKGE_CONTAINER_CLI", stub)
	t.Setenv("DOCKGE_LOG_LEVEL", "error")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	injector := do.New(
		func(i do.Injector) { do.ProvideValue(i, cfg) },
		log.Package,
		jwt.Package,
		func(i do.Injector) {
			do.Provide(i, func(i do.Injector) (*gin.Engine, error) { return gin.New(), nil })
		},
		repository.Package,
		service.Package,
		handler.Package,
		Package,
	)
	t.Cleanup(func() { _ = injector.Shutdown() })
	srv := do.MustInvoke[*httpx.Server](injector)
	return srv.Engine
}

// call 发起一次请求并返回状态码与解析后的响应包。
func call(t *testing.T, engine *gin.Engine, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	payload := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	return rec.Code, payload
}

// dataString 取响应 data 字段中的字符串键（data 为非对象时返回空串）。
func dataString(payload map[string]any, key string) string {
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return ""
	}
	if s, ok := data[key].(string); ok {
		return s
	}
	return ""
}

// TestAPI 覆盖接口契约：鉴权、栈、设置与已移除端点。
func TestAPI(t *testing.T) {
	engine := newTestServer(t)
	pass := "s3cret-pass"

	// ---- 未初始化：引导接口放行，其余一律 403 ----
	if code, payload := call(t, engine, "GET", "/v1/setup/need", "", nil); code != 200 || payload["data"].(map[string]any)["needSetup"] != true {
		t.Fatalf("setup/need = %d %v, want 200 needSetup=true", code, payload)
	}
	if code, _ := call(t, engine, "GET", "/v1/stacks", "", nil); code != http.StatusForbidden {
		t.Fatalf("未初始化访问受保护端点 = %d, want 403", code)
	}
	if code, _ := call(t, engine, "GET", "/v1/health", "", nil); code != 200 {
		t.Fatal("health 应始终放行")
	}
	if code, _ := call(t, engine, "GET", "/v1/robots.txt", "", nil); code != 200 {
		t.Fatal("robots.txt 应放行")
	}

	// ---- 初始化管理员 ----
	if code, payload := call(t, engine, "POST", "/v1/setup", "", map[string]string{"username": "admin", "password": pass}); code != 200 {
		t.Fatalf("setup = %d %v, want 200", code, payload)
	}
	if code, _ := call(t, engine, "POST", "/v1/setup", "", map[string]string{"username": "admin2", "password": pass}); code != http.StatusConflict {
		t.Fatalf("重复 setup 应 409，实际 %d", code)
	}
	if code, payload := call(t, engine, "GET", "/v1/setup/need", "", nil); code != 200 || payload["data"].(map[string]any)["needSetup"] != false {
		t.Fatal("初始化后 needSetup 应为 false")
	}

	// ---- 登录 ----
	if code, _ := call(t, engine, "POST", "/v1/login", "", map[string]string{"username": "admin", "password": "wrong"}); code != http.StatusUnauthorized {
		t.Fatal("错误密码应 401")
	}
	code, payload := call(t, engine, "POST", "/v1/login", "", map[string]string{"username": "admin", "password": pass})
	if code != 200 {
		t.Fatalf("登录 = %d %v, want 200", code, payload)
	}
	token := dataString(payload, "accessToken")
	if token == "" {
		t.Fatal("登录应返回 accessToken")
	}
	if code, _ := call(t, engine, "GET", "/v1/stacks", "", nil); code != http.StatusUnauthorized {
		t.Fatal("无 token 访问应 401")
	}
	if code, payload := call(t, engine, "GET", "/v1/me", token, nil); code != 200 || dataString(payload, "username") != "admin" {
		t.Fatalf("me = %d %v, want 200 admin", code, payload)
	}

	// ---- 栈：校验、创建、读取、保存、统计、网络、生命周期、删除 ----
	if code, _ := call(t, engine, "POST", "/v1/stacks", token, map[string]string{"name": "Bad Name", "yaml": "services: {}"}); code != http.StatusBadRequest {
		t.Fatal("非法栈名应 400")
	}
	if code, _ := call(t, engine, "POST", "/v1/stacks", token, map[string]string{"name": "demo", "yaml": "services: ["}); code != http.StatusBadRequest {
		t.Fatal("非法 YAML 应 400")
	}
	yamlBody := "services:\n  web:\n    image: nginx:latest\n"
	if code, payload := call(t, engine, "POST", "/v1/stacks", token, map[string]string{"name": "demo", "yaml": yamlBody, "env": "A=1\n"}); code != 200 {
		t.Fatalf("创建栈 = %d %v, want 200", code, payload)
	}
	if code, payload := call(t, engine, "GET", "/v1/stacks", token, nil); code != 200 || !strings.Contains(fmt.Sprint(payload["data"]), "demo") {
		t.Fatal("栈列表应包含新建栈")
	}
	if code, payload := call(t, engine, "GET", "/v1/stacks/demo", token, nil); code != 200 || dataString(payload, "yaml") != yamlBody {
		t.Fatalf("栈详情应回读 YAML，实际 %d %v", code, payload)
	}
	if code, _ := call(t, engine, "GET", "/v1/stacks/nonexistent", token, nil); code != http.StatusNotFound {
		t.Fatal("不存在的栈应 404")
	}
	if code, _ := call(t, engine, "PUT", "/v1/stacks/demo", token, map[string]string{"name": "demo", "yaml": yamlBody, "env": ""}); code != 200 {
		t.Fatal("保存草稿应 200")
	}
	if code, _ := call(t, engine, "GET", "/v1/stacks/demo/stats", token, nil); code != 200 {
		t.Fatal("资源统计应 200")
	}
	if code, payload := call(t, engine, "GET", "/v1/stacks/networks", token, nil); code != 200 || !strings.Contains(fmt.Sprint(payload["data"]), "bridge") {
		t.Fatalf("网络列表应含 bridge，实际 %d %v", code, payload)
	}
	if code, _ := call(t, engine, "POST", "/v1/stacks/demo/start", token, nil); code != 200 {
		t.Fatal("启动（流式）应 200")
	}
	if code, _ := call(t, engine, "POST", "/v1/stacks/demo/bogus-op", token, nil); code != http.StatusBadRequest {
		t.Fatal("未知操作应 400")
	}
	if code, _ := call(t, engine, "POST", "/v1/stacks/demo/services/web/restart", token, nil); code != 200 {
		t.Fatal("服务级重启应 200")
	}
	if code, _ := call(t, engine, "DELETE", "/v1/stacks/demo", token, nil); code != 200 {
		t.Fatal("删除栈应 200")
	}
	if code, _ := call(t, engine, "GET", "/v1/stacks/demo", token, nil); code != http.StatusNotFound {
		t.Fatal("删除后应 404")
	}

	// ---- 设置 ----
	if code, _ := call(t, engine, "GET", "/v1/settings/globalenv", token, nil); code != 200 {
		t.Fatal("globalenv 读取应 200")
	}
	if code, _ := call(t, engine, "PUT", "/v1/settings/globalenv", token, map[string]string{"content": "TZ=Asia/Shanghai\n"}); code != 200 {
		t.Fatal("globalenv 写入应 200")
	}
	if code, payload := call(t, engine, "GET", "/v1/settings/globalenv", token, nil); code != 200 || !strings.Contains(dataString(payload, "globalENV"), "Asia/Shanghai") {
		t.Fatal("globalenv 应回读写入内容")
	}
	if code, _ := call(t, engine, "PUT", "/v1/settings/primaryhostname", token, map[string]string{"hostname": "dockge.example.com"}); code != 200 {
		t.Fatal("主机名写入应 200")
	}
	if code, payload := call(t, engine, "GET", "/v1/settings/primaryhostname", token, nil); code != 200 || dataString(payload, "hostname") != "dockge.example.com" {
		t.Fatal("主机名应回读写入值")
	}

	// ---- 免登录模式：开启后可自动登录，关闭后拒绝 ----
	if code, payload := call(t, engine, "GET", "/v1/me/disableauth", token, nil); code != 200 || payload["data"].(map[string]any)["enabled"] != false {
		t.Fatal("免登录默认关闭")
	}
	if code, _ := call(t, engine, "POST", "/v1/me/disableauth", token, map[string]any{"enable": true, "currentPassword": pass}); code != 200 {
		t.Fatal("开启免登录应 200")
	}
	if code, _ := call(t, engine, "POST", "/v1/auto-login", "", nil); code != 200 {
		t.Fatal("免登录开启后 auto-login 应 200")
	}
	if code, _ := call(t, engine, "POST", "/v1/me/disableauth", token, map[string]any{"enable": false, "currentPassword": pass}); code != 200 {
		t.Fatal("关闭免登录应 200")
	}
	if code, _ := call(t, engine, "POST", "/v1/auto-login", "", nil); code != http.StatusUnauthorized {
		t.Fatal("免登录关闭后 auto-login 应 401")
	}

	// ---- 改密：旧密码失效，新密码可登录 ----
	const newPass = "n3w-pass-123"
	if code, _ := call(t, engine, "PUT", "/v1/me/password", token, map[string]string{"oldPassword": "wrong", "newPassword": newPass}); code != http.StatusBadRequest {
		t.Fatal("旧密码错误应 400")
	}
	if code, _ := call(t, engine, "PUT", "/v1/me/password", token, map[string]string{"oldPassword": pass, "newPassword": newPass}); code != 200 {
		t.Fatal("改密应 200")
	}
	if code, _ := call(t, engine, "POST", "/v1/login", "", map[string]string{"username": "admin", "password": pass}); code != http.StatusUnauthorized {
		t.Fatal("旧密码应失效")
	}
	code, payload = call(t, engine, "POST", "/v1/login", "", map[string]string{"username": "admin", "password": newPass})
	if code != 200 {
		t.Fatal("新密码应可登录")
	}
	// 改密使绑定密码哈希的旧 token 立即失效（CheckSession 设计），后续断言用新会话
	if token = dataString(payload, "accessToken"); token == "" {
		t.Fatal("重新登录应返回 accessToken")
	}
	if code, _ := call(t, engine, "GET", "/v1/me", token, nil); code != 200 {
		t.Fatal("改密后新会话应可用")
	}

	// ---- 其他接口 ----
	if code, _ := call(t, engine, "GET", "/v1/health", "", nil); code != 200 {
		t.Fatal("health 应 200")
	}
	if code, payload := call(t, engine, "POST", "/v1/composerize", token, map[string]string{"dockerRunCommand": "docker run -d --name nginx -p 8080:80 nginx"}); code != 200 || !strings.Contains(dataString(payload, "composeTemplate"), "services:") {
		t.Fatalf("composerize 应返回 compose 模板，实际 %d %v", code, payload)
	}
	if code, _ := call(t, engine, "POST", "/v1/composerize", token, map[string]string{"dockerRunCommand": ""}); code != http.StatusBadRequest {
		t.Fatal("空命令应 400")
	}

	// ---- 已移除端点必须是 404（避免误复活） ----
	for _, removed := range []struct{ method, path string }{
		{"GET", "/v1/agents"},
		{"GET", "/v1/console/enabled"},
		{"GET", "/v1/console/terminal"},
		{"POST", "/v1/stacks/validate"},
		{"GET", "/v1/oidc/providers"},
		{"GET", "/v1/auth/config"},
		{"GET", "/v1/version/check"},
	} {
		if code, _ := call(t, engine, removed.method, removed.path, token, map[string]string{}); code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404（该端点应已移除）", removed.method, removed.path, code)
		}
	}

	// ---- 静态与 SPA 回退 ----
	if code, _ := call(t, engine, "GET", "/", "", nil); code != 200 {
		t.Fatal("SPA shell 应 200")
	}
	if code, payload := call(t, engine, "GET", "/v1/definitely-not-a-route", token, nil); code != http.StatusNotFound || !strings.Contains(fmt.Sprint(payload["message"]), "接口不存在") {
		t.Fatalf("未知 API 路径应返回 404 JSON，实际 %d %v", code, payload)
	}
}
