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
						Ports []struct {
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
	c := spec.Containers[0]
	if c.Ports[0].Port != 5001 {
		t.Fatal("fixed port changed")
	}
	paths := map[string]bool{}
	for _, m := range c.Mounts {
		paths[m.MountPath] = true
		if strings.HasPrefix(m.MountPath, "/host/") && !m.ReadOnly {
			t.Fatalf("host metrics mount %s must be read-only", m.MountPath)
		}
	}
	for _, path := range []string{"/opt/stacks", "/app/data", "/var/run/docker.sock", "/host/proc/stat", "/host/proc/meminfo", "/host/proc/cpuinfo", "/host/proc/loadavg", "/host/proc/uptime", "/host/proc/sys/kernel/osrelease", "/host/etc/os-release", "/host/etc/hostname"} {
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
