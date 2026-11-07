package deploy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultComposeHasOnlyTwoMounts(t *testing.T) {
	b, err := os.ReadFile("compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services map[string]struct{ Volumes []string }
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	mounts := doc.Services["dockge"].Volumes
	if len(mounts) != 2 || mounts[0] != "/var/run/docker.sock:/var/run/docker.sock" || mounts[1] != "../stacks:/opt/stacks" {
		t.Fatalf("%v", mounts)
	}
	b, err = os.ReadFile("Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "VOLUME /app/data") {
		t.Fatal("匿名卷会覆盖隐藏数据目录，不能保留")
	}
}

// TestEntrypointDataPersistence 在隔离目录执行真实 shell，检查重建、旧挂载与拒绝覆盖。
func TestEntrypointDataPersistence(t *testing.T) {
	for _, mode := range []string{"new", "old-mount", "old-unmounted", "missing-stacks", "linked-data"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			stacks, data, mounts := filepath.Join(root, "stacks"), filepath.Join(root, "data"), filepath.Join(root, "mountinfo")
			if err := os.Mkdir(stacks, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(data, 0o755); err != nil {
				t.Fatal(err)
			}
			mountinfo := "1 0 0:1 / " + stacks + " rw - ext4 /dev/test rw\n"
			if mode == "missing-stacks" {
				mountinfo = ""
			}
			if mode == "old-mount" {
				mountinfo += "2 0 0:2 / " + data + " rw - ext4 /dev/test rw\n"
			}
			if strings.HasPrefix(mode, "old-") {
				if err := os.WriteFile(filepath.Join(data, "account"), []byte("existing"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "linked-data" {
				if err := os.Symlink(data, filepath.Join(stacks, ".dockge-data")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(mounts, []byte(mountinfo), 0o600); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile("entrypoint.sh")
			if err != nil {
				t.Fatal(err)
			}
			script := strings.NewReplacer("/opt/stacks", stacks, "/app/data", data, "/proc/self/mountinfo", mounts).Replace(string(b))
			script = strings.Replace(script, "exec dockge-server \"$@\"", "printf started >> '"+data+"/starts'", 1)
			run := func() ([]byte, error) { return exec.Command("sh", "-c", script).CombinedOutput() }
			out, err := run()
			if mode == "old-unmounted" || mode == "missing-stacks" || mode == "linked-data" {
				if err == nil || !strings.Contains(string(out), "Dockge:") {
					t.Fatalf("必须明确拒绝: %s %v", out, err)
				}
				if mode == "old-unmounted" {
					if b, err := os.ReadFile(filepath.Join(data, "account")); err != nil || string(b) != "existing" {
						t.Fatal("旧账号不得改动")
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("%s %v", out, err)
			}
			if mode == "new" {
				if target, err := os.Readlink(data); err != nil || target != filepath.Join(stacks, ".dockge-data") {
					t.Fatalf("固定 data 路径必须连接持久目录: %s %v", target, err)
				}
				if err := os.Remove(data); err != nil {
					t.Fatal(err)
				} // 模拟新容器没有旧软链。
			}
			if out, err := run(); err != nil {
				t.Fatalf("重建失败: %s %v", out, err)
			}
			if b, err := os.ReadFile(filepath.Join(data, "starts")); err != nil || string(b) != "startedstarted" {
				t.Fatal("重建不能丢数据")
			}
		})
	}
}
