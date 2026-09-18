package service

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseComposeErrors(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []struct {
			line int
			msg  string
		}
	}{
		{
			name: "yaml 类错误提取行号",
			out:  "yaml: line 3: could not find expected ':'",
			want: []struct {
				line int
				msg  string
			}{{3, "yaml: line 3: could not find expected ':'"}},
		},
		{
			name: "纯语义错误无行号降级为零",
			out:  "services.web.ports must be a list",
			want: []struct {
				line int
				msg  string
			}{{0, "services.web.ports must be a list"}},
		},
		{
			name: "剥离 validating 临时路径前缀",
			out:  "validating /tmp/dockge-validate-123/compose.yaml: services.web.image must be a string",
			want: []struct {
				line int
				msg  string
			}{{0, "services.web.image must be a string"}},
		},
		{
			name: "多行输出逐行拆分且跳过空行",
			out:  "validating /tmp/x/compose.yaml: services.web.image must be a string\n\nyaml: line 9: mapping values are not allowed in this context",
			want: []struct {
				line int
				msg  string
			}{
				{0, "services.web.image must be a string"},
				{9, "yaml: line 9: mapping values are not allowed in this context"},
			},
		},
		{
			name: "空输出返回空切片",
			out:  "",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseComposeErrors(tt.out)
			if len(got) != len(tt.want) {
				t.Fatalf("parseComposeErrors(%q) 返回 %d 条, 期望 %d 条: %+v", tt.out, len(got), len(tt.want), got)
			}
			for i, w := range tt.want {
				if got[i].Line != w.line || got[i].Message != w.msg {
					t.Errorf("第 %d 条 = {Line:%d, Message:%q}, 期望 {Line:%d, Message:%q}",
						i, got[i].Line, got[i].Message, w.line, w.msg)
				}
			}
		})
	}
}

func TestYAMLErrorLine(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want int
	}{
		{"语法错误返回行号", "services:\n  web\n    image: nginx\n", 3},
		{"合法 yaml 返回零", "services:\n  web:\n    image: nginx\n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc any
			err := yaml.Unmarshal([]byte(tt.yaml), &doc)
			got := yamlErrorLine(err)
			if got != tt.want {
				t.Fatalf("yamlErrorLine(%q) = %d, 期望 %d (err=%v)", tt.yaml, got, tt.want, err)
			}
		})
	}
}
