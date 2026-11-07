package kubernetes

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDeploymentContract 不需要集群也能防止内部路径、副本数、认证注入和权限约束退化。
func TestDeploymentContract(t *testing.T) {
	raw, err := os.ReadFile("deployment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Spec struct {
			Replicas int
			Strategy struct{ Type string }
			Template struct {
				Spec struct {
					Automount  bool              `yaml:"automountServiceAccountToken"`
					Selector   map[string]string `yaml:"nodeSelector"`
					Containers []struct {
						Image      string
						PullPolicy string `yaml:"imagePullPolicy"`
						Readiness  struct {
							HTTPGet struct {
								Path string
								Port string
							} `yaml:"httpGet"`
						} `yaml:"readinessProbe"`
						Liveness struct {
							HTTPGet struct {
								Path string
								Port string
							} `yaml:"httpGet"`
						} `yaml:"livenessProbe"`
						Ports []struct {
							Name string
							Port int `yaml:"containerPort"`
						}
						EnvFrom []struct {
							ConfigMapRef *struct{ Name string } `yaml:"configMapRef"`
							SecretRef    *struct{ Name string } `yaml:"secretRef"`
						} `yaml:"envFrom"`
						Mounts []struct {
							MountPath string `yaml:"mountPath"`
							ReadOnly  bool   `yaml:"readOnly"`
						} `yaml:"volumeMounts"`
					}
				}
			}
		}
	}
	if err := yaml.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	spec := d.Spec.Template.Spec
	if d.Spec.Replicas != 1 || d.Spec.Strategy.Type != "Recreate" || spec.Automount || spec.Selector["dockge-runtime"] != "true" {
		t.Fatal("single-node or least-privilege contract broken")
	}
	if len(spec.Containers) != 1 {
		t.Fatalf("expected one panel container, got %d", len(spec.Containers))
	}
	c := spec.Containers[0]
	if c.Image != "makeshit/dockge:0.2.6" || c.PullPolicy != "IfNotPresent" {
		t.Fatalf("两挂载部署必须固定兼容的发布镜像并允许拉取: image=%q, policy=%q", c.Image, c.PullPolicy)
	}
	for name, probe := range map[string]struct{ Path, Port string }{
		"readiness": c.Readiness.HTTPGet, "liveness": c.Liveness.HTTPGet,
	} {
		if probe.Path != "/v1/health" || probe.Port != "http" {
			t.Fatalf("%s must use the public health endpoint and named HTTP port: %+v", name, probe)
		}
	}
	if len(c.Ports) != 1 || c.Ports[0].Name != "http" || c.Ports[0].Port != 5001 {
		t.Fatal("fixed port changed")
	}
	paths := map[string]bool{}
	for _, m := range c.Mounts {
		paths[m.MountPath] = true
		if strings.HasPrefix(m.MountPath, "/host/") && !m.ReadOnly {
			t.Fatalf("host metrics mount %s must be read-only", m.MountPath)
		}
	}
	if len(paths) != 2 {
		t.Fatalf("默认仅允许两个挂载: %v", paths)
	}
	for _, path := range []string{"/opt/stacks", "/var/run/docker.sock"} {
		if !paths[path] {
			t.Fatalf("missing fixed mount %s", path)
		}
	}
	hasSecret, hasConfig := false, false
	for _, e := range c.EnvFrom {
		if e.SecretRef != nil && e.SecretRef.Name == "dockge-oidc" {
			hasSecret = true
		}
		if e.ConfigMapRef != nil && e.ConfigMapRef.Name == "dockge-config" {
			hasConfig = true
		}
	}
	if !hasSecret || !hasConfig {
		t.Fatal("missing authentication injection")
	}
	for _, f := range []string{"configmap.yaml", "secret.example.yaml", "namespace.yaml", "service.yaml", "ingress.example.yaml"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var node yaml.Node
		if err := yaml.Unmarshal(raw, &node); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
}
