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
