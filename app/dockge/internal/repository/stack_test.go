package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dockge/app/dockge/internal/model"
)

// TestTrimStackOutput 校验 compose 输出的首尾空白行规整。
func TestTrimStackOutput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"blank edges", "\n\na\nb\n\n\n", "a\nb"},
		{"no trim needed", "a\nb", "a\nb"},
		{"only blanks", "\n\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrimStackOutput(tt.in); got != tt.want {
				t.Errorf("TrimStackOutput(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestSaveRemovesStaleEnv 校验保存时 env 为空须删除残留 .env：
// 否则 UI 清空后旧文件复活，compose 读取坏内容导致栈永远无法启动。
func TestSaveRemovesStaleEnv(t *testing.T) {
	r := &Repository{stacksDir: t.TempDir()}
	ctx := context.Background()
	stack := &model.Stack{Name: "te", Yaml: "services: {}\n", Env: "A=1\n", ComposeFileName: "compose.yaml"}
	if err := r.Save(ctx, stack, true); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	stack.Env = ""
	if err := r.Save(ctx, stack, false); err != nil {
		t.Fatalf("save with empty env: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.stacksDir, "te", ".env")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale .env should be removed after saving empty env, stat err = %v", err)
	}
}

// TestParseXDockgeURLs 校验 x-dockge.urls 扩展字段解析与 ${VAR} 替换。
func TestParseXDockgeURLs(t *testing.T) {
	yaml := `
x-dockge:
  urls:
    - http://localhost:${WEB_PORT}
    - https://example.com
services: {}
`
	env := "WEB_PORT=8080"
	got := ParseXDockgeURLs(yaml, env)
	if len(got) != 2 || got[0] != "http://localhost:8080" || got[1] != "https://example.com" {
		t.Errorf("ParseXDockgeURLs = %v", got)
	}
	if urls := ParseXDockgeURLs("services: {}", ""); len(urls) != 0 {
		t.Errorf("no urls expected, got %v", urls)
	}
}

// TestGetExternalStackCallsComposeLsOnce 用 PATH 桩替身 docker CLI，
// 锁定外部栈 Get 的行为：状态来自 compose ls 且整个 Get 期间
// 「状态查询」只发一次 compose ls（外部分支 + StackPs 各一次），
// 防止回归成同函数内重复查询。
func TestGetExternalStackCallsComposeLsOnce(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "ls.count")

	// 桩 docker：compose ls 输出一个外部栈并对调用计数；其余 compose 子命令返回空列表。
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = compose ] && [ \"$2\" = ls ]; then\n" +
		"printf x >> \"$DOCKER_LS_COUNTER\"\n" +
		"echo '[{\"Name\":\"ext-stack\",\"Status\":\"running(2)\",\"ConfigFiles\":\"/nonexistent/compose.yaml\"}]'\n" +
		"exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = compose ]; then echo '[]'; exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_LS_COUNTER", counter)

	stacksDir := filepath.Join(dir, "stacks")
	if err := os.MkdirAll(stacksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	repo := &Repository{stacksDir: stacksDir}

	st, err := repo.Get(context.Background(), "ext-stack")
	if err != nil {
		t.Fatalf("Get(ext-stack) error: %v", err)
	}
	if st.Managed {
		t.Error("external stack should not be managed")
	}
	if st.Status != model.StatusRunning {
		t.Errorf("status = %d, want %d (running)", st.Status, model.StatusRunning)
	}

	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	// 外部栈分支 1 次 + StackPs 的 resolveStackExec 1 次 = 2；重复查询回归会变成 3。
	if n := len(data); n != 2 {
		t.Errorf("compose ls called %d times, want 2 (no duplicate status query)", n)
	}
}
