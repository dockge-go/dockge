package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockge/internal/model"
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

// TestGetManagedDraft 校验详情与列表的草稿状态一致，环境文件读取失败不能伪装为空。
func TestGetManagedDraft(t *testing.T) {
	for _, badEnv := range []bool{false, true} {
		t.Run(fmt.Sprintf("badEnv=%v", badEnv), func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "runtime")
			if err := os.WriteFile(bin, []byte("#!/bin/sh\necho '[]'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			r := &Repository{stacksDir: dir, runtime: Runtime{bin: bin, composeBin: bin, composeSub: []string{}}}
			stack := &model.Stack{Name: "draft", Yaml: "services: {}\n"}
			if err := r.Save(context.Background(), stack, true); err != nil {
				t.Fatal(err)
			}
			if badEnv {
				if err := os.Mkdir(filepath.Join(dir, "draft", ".env"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			got, err := r.Get(context.Background(), "draft")
			if badEnv {
				if err == nil || !strings.Contains(err.Error(), ".env") || !strings.Contains(err.Error(), "draft") {
					t.Fatalf("expected environment read details, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != model.StatusCreatedFile || got.Yaml != stack.Yaml {
				t.Fatalf("draft mismatch: %+v", got)
			}
		})
	}
}

// TestGetManagedInvalidComposeKeepsEditableFile 防止状态查询失败遮蔽可修正的 YAML。
func TestGetManagedInvalidComposeKeepsEditableFile(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "runtime")
	script := "#!/bin/sh\nfor arg; do\nif [ \"$arg\" = ps ]; then\necho 'services.app Additional property unsupported_field is not allowed' >&2\nexit 1\nfi\ndone\necho '[]'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Repository{stacksDir: dir, runtime: Runtime{bin: bin, composeBin: bin, composeSub: []string{}}}
	stack := &model.Stack{Name: "invalid", Yaml: "services:\n  app:\n    image: busybox:1.37.0\n    unsupported_field: true\n", Env: "A=1\n"}
	if err := r.Save(context.Background(), stack, true); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(context.Background(), stack.Name)
	if err != nil {
		t.Fatalf("运行状态失败不应遮蔽可编辑的真实文件: %v", err)
	}
	if !got.Managed || got.Yaml != stack.Yaml || got.Env != stack.Env || got.Status != model.StatusUnknown {
		t.Fatalf("配置或未知运行状态不正确: %+v", got)
	}
	if !strings.Contains(got.RuntimeError, "unsupported_field") || !strings.Contains(got.RuntimeError, stack.Name) {
		t.Fatalf("未保留具体运行时错误: %q", got.RuntimeError)
	}
}

// TestListRuntimeFailurePreservesFilesAsUnknown 防止运行时不可达时把已有栈误报为未部署。
func TestListRuntimeFailurePreservesFilesAsUnknown(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(fmt.Sprintf("unavailable=%v", unavailable), func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "runtime")
			script := "#!/bin/sh\necho '[]'\n"
			if unavailable {
				script = "#!/bin/sh\necho 'runtime socket permission denied' >&2\nexit 1\n"
			}
			if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			r := &Repository{stacksDir: dir, runtime: DetectRuntime(bin, "")}
			stack := &model.Stack{Name: "editable", Yaml: "services: {}\n"}
			if err := r.Save(context.Background(), stack, true); err != nil {
				t.Fatal(err)
			}
			rows, err := r.List(context.Background())
			if err != nil || len(rows) != 1 || !rows[0].Managed {
				t.Fatalf("文件列表不可丢失：%+v, %v", rows, err)
			}
			if unavailable {
				if rows[0].Status != model.StatusUnknown || !strings.Contains(rows[0].RuntimeError, "runtime socket permission denied") {
					t.Fatalf("未知状态必须含真实错误：%+v", rows[0])
				}
			} else if rows[0].Status != model.StatusCreatedFile || rows[0].RuntimeError != "" {
				t.Fatalf("查询成功才可确认仅有文件：%+v", rows[0])
			}
		})
	}
}
