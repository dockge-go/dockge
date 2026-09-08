package repository

import "testing"

// TestSplitRepoTag 校验镜像引用的仓库/标签拆分（registry 端口冒号不误切）。
func TestSplitRepoTag(t *testing.T) {
	tests := []struct {
		repoTag string
		repo    string
		tag     string
	}{
		{"nginx:alpine", "nginx", "alpine"},
		{"nginx", "nginx", "latest"},
		{"registry:5000/app/web:v2", "registry:5000/app/web", "v2"},
		{"<none>", "<none>", "latest"},
	}
	for _, tt := range tests {
		repo, tag := splitRepoTag(tt.repoTag)
		if repo != tt.repo || tag != tt.tag {
			t.Errorf("splitRepoTag(%q) = (%q, %q), want (%q, %q)", tt.repoTag, repo, tag, tt.repo, tt.tag)
		}
	}
}

// TestFormatBytes 校验字节的人类可读格式化。
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes float64
		want  string
	}{
		{0, "0.0B"},
		{512, "512.0B"},
		{1024 * 1024, "1.0MB"},
		{1.5 * 1024 * 1024 * 1024, "1.5GB"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.bytes); got != tt.want {
			t.Errorf("formatBytes(%v) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
