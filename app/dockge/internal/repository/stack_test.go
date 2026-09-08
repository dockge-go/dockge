package repository

import "testing"

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
