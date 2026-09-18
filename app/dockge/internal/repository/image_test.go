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

// TestImageRefPattern 锁定镜像引用校验：带 tag/digest/registry 端口的合法引用
// MUST 放行（曾因复用容器 ID 模式而全数误拒），注入元字符 MUST 拒绝。
func TestImageRefPattern(t *testing.T) {
	valid := []string{
		"hello-world", "hello-world:latest", "nginx:alpine",
		"registry.example.com:5000/app/web:v1.2", "busybox@sha256:abc123def",
		"docker.io/library/redis:7-alpine",
	}
	invalid := []string{
		"", "nginx:alpine; rm -rf /", "a b", "nginx$(id)", "nginx`id`", ":tag",
	}
	for _, ref := range valid {
		if !ImageRefPattern.MatchString(ref) {
			t.Errorf("ImageRefPattern 拒绝了合法引用 %q", ref)
		}
	}
	for _, ref := range invalid {
		if ImageRefPattern.MatchString(ref) {
			t.Errorf("ImageRefPattern 放行了非法引用 %q", ref)
		}
	}
}
