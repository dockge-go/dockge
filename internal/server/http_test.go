package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockge/internal/handler"
	"dockge/internal/repository"
	"dockge/internal/service"
	"dockge/pkg/config"
	"dockge/pkg/jwt"
	"dockge/pkg/log"

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
if [ "$1" = compose ] && [ "$6" = config ]; then
 echo '{"services":{"web":{"image":"nginx:latest"}},"volumes":{},"networks":{}}'
 exit 0
fi
case "$*" in
  "compose ls --all --format json") echo '[]' ;;
  "network ls --format {{.Name}}") echo "bridge host none mynet" ;;
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
		func(i do.Injector) {
			do.Provide(i, func(i do.Injector) (*log.Logger, error) { return log.NewLogger(cfg), nil })
		},
		func(i do.Injector) {
			do.Provide(i, func(i do.Injector) (*jwt.JWT, error) { return jwt.NewJWT(cfg), nil })
		},
		repository.Package,
		service.Package,
		handler.Package,
		Package,
	)
	t.Cleanup(func() { _ = injector.Shutdown() })
	srv := do.MustInvoke[*HTTPServer](injector)
	return srv.engine
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
	if code, payload := call(t, engine, "GET", "/v1/stacks/networks", token, nil); code != 200 || !strings.Contains(fmt.Sprint(payload["data"]), "mynet") || strings.Contains(fmt.Sprint(payload["data"]), "bridge") {
		t.Fatalf("网络列表应含 mynet 且滤除内置 bridge/host/none，实际 %d %v", code, payload)
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
	if code, payload := call(t, engine, "GET", "/v1/stacks/demo/delete-plan", token, nil); code != 200 {
		t.Fatalf("删除预检应 200: %d %v", code, payload)
	}
	if code, payload := call(t, engine, "DELETE", "/v1/stacks/demo", token, nil); code != 200 {
		t.Fatalf("删除栈应 200: %d %v", code, payload)
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
		{"GET", "/v1/users"},
		{"POST", "/v1/users"},
		{"PUT", "/v1/users/other/password"},
		{"DELETE", "/v1/users/other"},
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

// TestHTTPRecovery 验证生产装配捕获未写响应的 panic，并保留定位所需的动作与原因。
func TestHTTPRecovery(t *testing.T) {
	e := newTestServer(t)
	e.GET("/panic-regression", func(c *gin.Context) { panic("test panic cause") })
	w := httptest.NewRecorder()
	var caught any
	func() {
		defer func() { caught = recover() }()
		e.ServeHTTP(w, httptest.NewRequest("GET", "/panic-regression", nil))
	}()
	if caught != nil {
		t.Fatalf("panic 逃出 HTTP 引擎：%v", caught)
	}
	if w.Code != 500 || !json.Valid(w.Body.Bytes()) || !strings.Contains(w.Body.String(), "test panic cause") {
		t.Fatalf("panic 响应 = %d %s", w.Code, w.Body.String())
	}
}

// TestHTTPContextCancellation 保证传入 service 的 Gin context 能感知底层请求被取消。
func TestHTTPContextCancellation(t *testing.T) {
	e := newTestServer(t)
	e.GET("/context-regression", func(c *gin.Context) {
		if !errors.Is(c.Err(), context.Canceled) || c.Done() == nil {
			t.Errorf("请求取消未传递：Err=%v Done=%v", c.Err(), c.Done())
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/context-regression", nil).WithContext(ctx))
}

// TestHTTPShutdownDeadline 超时退出后仍须关闭活动连接，使请求 context 取消。
func TestHTTPShutdownDeadline(t *testing.T) {
	entered, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
		close(stopped)
	}))
	defer srv.Close()
	defer close(release)
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		res, err := srv.Client().Get(srv.URL)
		if err == nil {
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("请求未进入")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := (&HTTPServer{Server: srv.Config}).Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown=%v", err)
	}
	select {
	case <-stopped:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Shutdown 超时后活动连接/请求未关闭")
	}
	select {
	case <-clientDone:
	case <-time.After(time.Second):
		t.Fatal("客户端连接未退出")
	}
}

// TestLoginForwardedHeadersCannotBypassLimit 未配置可信代理时不能靠伪造头切换限流键。
func TestLoginForwardedHeadersCannotBypassLimit(t *testing.T) {
	e := newTestServer(t)
	if code, payload := call(t, e, "POST", "/v1/setup", "", map[string]string{"username": "admin", "password": "s3cret-pass"}); code != 200 {
		t.Fatalf("setup=%d %v", code, payload)
	}
	for n := 0; n < 11; n++ {
		req := httptest.NewRequest("POST", "/v1/login", strings.NewReader(`{}`))
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", n+1))
		req.Header.Set("X-Real-IP", fmt.Sprintf("203.0.113.%d", n+1))
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		want := 400
		if n == 10 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("第 %d 次伪造头请求=%d %s，期望 %d", n+1, w.Code, w.Body.String(), want)
		}
	}
}
