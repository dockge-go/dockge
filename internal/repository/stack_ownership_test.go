package repository

import (
	"context"
	"dockge/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStackOwnershipBlocksOutsideProject(t *testing.T) {
	r, root := fixtureRuntime(t)
	putFixture(t, filepath.Join(root, "container.json"), `[{"Id":"cid","Config":{"Labels":{"com.docker.compose.project":"test","com.docker.compose.project.config_files":"/outside/compose.yaml"}}}]`)
	if err := r.checkStackOwnership(context.Background(), "test"); err == nil {
		t.Fatal("同名目录外项目未阻止")
	}
	for _, input := range []string{"", "/outside/compose.yaml", r.StackPath("test") + "/compose.yaml,/outside/compose.yaml"} {
		if r.configOwned("test", input) {
			t.Fatalf("误认归属 %q", input)
		}
	}
	if !r.configOwned("test", r.StackPath("test")+"/compose.yaml") {
		t.Fatal("托管文件未识别")
	}
}

func TestSelfContainerIdentityCustomHostname(t *testing.T) {
	id := strings.Repeat("a", 64)
	for _, tc := range []struct{ hostname, cgroup, mounts, want string }{
		{"custom", "0::/system.slice/docker-" + id + ".scope", "", id},
		{"custom", "0::/", "25 23 0:20 /var/lib/docker/containers/" + id + "/hostname /etc/hostname rw - ext4 /dev/x rw", id},
		{id[:12], "", "", id[:12]}, {"server-host", "0::/", "", ""},
	} {
		if got := containerIdentity(tc.hostname, tc.cgroup, tc.mounts); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
}

func TestSaveFailurePreservesYAMLAndEnv(t *testing.T) {
	r, root := fixtureRuntime(t)
	path := r.StackPath("test")
	putFixture(t, filepath.Join(path, ".env"), "ORIGINAL=1")
	old, _ := os.ReadFile(filepath.Join(path, "compose.yaml"))
	// 保存目标是目录时必须在变更 .env 之前拒绝。
	if err := os.Remove(filepath.Join(path, "compose.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(path, "compose.yaml"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(context.Background(), &model.Stack{Name: "test", Yaml: "new", Env: "CHANGED=1"}, false); err == nil {
		t.Fatal("目录保存未拒绝")
	}
	env, _ := os.ReadFile(filepath.Join(path, ".env"))
	if string(env) != "ORIGINAL=1" {
		t.Fatal("失败后 .env 被修改")
	}
	_ = os.Remove(filepath.Join(path, "compose.yaml"))
	putFixture(t, filepath.Join(path, "compose.yaml"), string(old))
	// 环境目标符号链接必须拒绝，不能改栈外文件。
	_ = os.Remove(filepath.Join(path, ".env"))
	external := filepath.Join(root, "outside-env")
	putFixture(t, external, "KEEP")
	if err := os.Symlink(external, filepath.Join(path, ".env")); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(context.Background(), &model.Stack{Name: "test", Yaml: "new", Env: "CHANGED=1"}, false); err == nil {
		t.Fatal("符号链接保存未拒绝")
	}
	yaml, _ := os.ReadFile(filepath.Join(path, "compose.yaml"))
	env, _ = os.ReadFile(external)
	if string(yaml) != string(old) || string(env) != "KEEP" {
		t.Fatal("失败保存产生了部分变更")
	}
	if err := r.Save(context.Background(), &model.Stack{Name: "new", ComposeFileName: "invalid"}, true); err == nil {
		t.Fatal("非法文件名未拒绝")
	}
	if _, err := os.Stat(r.StackPath("new")); !os.IsNotExist(err) {
		t.Fatal("新建失败遗留空栈")
	}
}

func TestBindSharedSymlinkIsInsideStack(t *testing.T) {
	r, _ := fixtureRuntime(t)
	link := filepath.Join(t.TempDir(), "shared")
	if err := os.Symlink(r.StackPath("test"), link); err != nil {
		t.Fatal(err)
	}
	if !r.stackSourceInside("test", link) {
		t.Fatal("外部符号链接指向栈内共享目录未识别")
	}
	if r.stackSourceInside("test", t.TempDir()) {
		t.Fatal("外部目录误判栈内")
	}
}

// TestStackOwnershipRejectsLinkedConfigFile 路径在栈内不等于配置属于栈，符号链接不得成为清理归属证明。
func TestStackOwnershipRejectsLinkedConfigFile(t *testing.T) {
	r, root := fixtureRuntime(t)
	outside := filepath.Join(root, "outside-compose.yaml")
	putFixture(t, outside, "services: {}")
	linked := filepath.Join(r.StackPath("test"), "docker-compose.yml")
	if err := os.Symlink(outside, linked); err != nil {
		t.Fatal(err)
	}
	putFixture(t, filepath.Join(root, "container.json"), jsonFixture(t, []map[string]interface{}{{
		"Id": "outside-container",
		"Config": map[string]interface{}{"Labels": map[string]string{
			"com.docker.compose.project":              "test",
			"com.docker.compose.project.config_files": linked,
		}},
	}}))
	if _, err := r.CleanStackResources(context.Background(), "test"); err == nil || !strings.Contains(err.Error(), linked) {
		t.Errorf("链接配置不能证明归属，必须返回实际路径：%v", err)
	}
	commands, err := os.ReadFile(filepath.Join(root, "commands"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(commands), " down ") {
		t.Errorf("归属不明时执行了 down：%s", commands)
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "services: {}" {
		t.Fatalf("栈外配置被改变：%q, %v", data, err)
	}
}
