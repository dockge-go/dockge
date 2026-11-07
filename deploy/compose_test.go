package deploy

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestComposeDeployment 防止标准部署再次增加必需挂载或改变容器内部路径。
func TestComposeDeployment(t *testing.T) {
	b, err := os.ReadFile("compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Services map[string]struct {
			Image         string
			ContainerName string `yaml:"container_name"`
			Ports         []string
			Volumes       []string
			Environment   map[string]string
			Privileged    bool
		}
	}
	if err := yaml.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}
	s, ok := manifest.Services["dockge"]
	if !ok || len(manifest.Services) != 1 {
		t.Fatal("仅部署本机面板服务")
	}
	if s.Image == "" || s.ContainerName != "dockge" || len(s.Ports) != 1 || s.Ports[0] != "5001:5001" {
		t.Fatalf("服务契约发生变化: %+v", s)
	}
	want := map[string]bool{"/var/run/docker.sock:/var/run/docker.sock": true, "../stacks:/opt/stacks": true}
	if len(s.Volumes) != len(want) {
		t.Fatalf("标准部署只需 socket 和栈目录: %v", s.Volumes)
	}
	for _, v := range s.Volumes {
		if !want[v] {
			t.Fatalf("未知挂载: %s", v)
		}
	}
	if len(s.Environment) != 0 || s.Privileged {
		t.Fatal("标准部署不需要路径环境覆盖或特权容器")
	}
}
