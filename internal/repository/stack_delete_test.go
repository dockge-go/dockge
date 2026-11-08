package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockge/internal/model"
)

func putFixture(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}
func jsonFixture(t *testing.T, v interface{}) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

// fixtureRuntime 仅执行临时目录脚本，测试不会接触本机运行时或已有容器。
func fixtureRuntime(t *testing.T) (*Repository, string) {
	t.Helper()
	root := t.TempDir()
	stacks := filepath.Join(root, "stacks")
	if err := os.MkdirAll(filepath.Join(stacks, "test"), 0755); err != nil {
		t.Fatal(err)
	}
	putFixture(t, filepath.Join(stacks, "test", "compose.yaml"), "services: {}")
	putFixture(t, filepath.Join(stacks, "test", "resolved.json"), `{"services":{"app":{"image":"test:1","volumes":[]}},"volumes":{},"networks":{}}`)
	for _, kind := range []string{"container", "volume", "network", "image"} {
		putFixture(t, filepath.Join(root, kind+".json"), "[]")
	}
	bin := filepath.Join(root, "runtime")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$FIXTURE_ROOT/commands"
if [ "$1" = compose ]; then
 if [ "$2" = ls ]; then printf '[{"Name":"test","Status":"running(1)","ConfigFiles":"%s/stacks/test/compose.yaml"}]' "$FIXTURE_ROOT"; exit 0; fi
 if [ "$6" = config ]; then cat "$PWD/resolved.json"; exit 0; fi
 if [ "$6" = down ]; then
  if [ -f "$FIXTURE_ROOT/fail-down" ]; then echo 'permission denied on test resource' >&2; exit 1; fi
  touch "$FIXTURE_ROOT/after"
  exit 0
 fi
fi
if [ "$1" = ps ]; then
 if [ ! -f "$FIXTURE_ROOT/after" ] || [ -f "$FIXTURE_ROOT/orphan" ]; then echo cid; fi
 exit 0
fi
if [ "$1" = inspect ]; then cat "$FIXTURE_ROOT/container.json"; exit 0; fi
kind="$1"
if [ "$1" = unpause ]; then
 if [ -f "$FIXTURE_ROOT/fail-unpause" ]; then echo 'unpause denied' >&2; exit 1; fi
 exit 0
fi
if [ "$1" = builder ] && [ "$2" = prune ]; then echo 'Total reclaimed space: 1MB'; exit 0; fi
if [ "$1" = stop ]; then
 if [ -f "$FIXTURE_ROOT/fail-stop" ]; then echo 'orphan stop denied' >&2; exit 1; fi
 exit 0
fi
if [ "$1" = rm ]; then
 printf '[]' > "$FIXTURE_ROOT/container.json"
 rm -f "$FIXTURE_ROOT/orphan"
 exit 0
fi
if [ "$2" = ls ]; then echo item; exit 0; fi
if [ "$2" = inspect ]; then
 if [ -f "$FIXTURE_ROOT/after" ] && [ "$kind" != image ]; then echo '[]'; else cat "$FIXTURE_ROOT/$kind.json"; fi
 exit 0
fi
if [ "$2" = rm ]; then
 if [ "$kind" = image ] && [ ! -f "$FIXTURE_ROOT/keep-image" ] && [ "$3" = test:2 ]; then printf '[]' > "$FIXTURE_ROOT/image.json"; fi
 exit 0
fi
echo "unexpected command $*" >&2; exit 1
`
	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FIXTURE_ROOT", root)
	return &Repository{stacksDir: stacks, runtime: Runtime{bin: bin, composeBin: bin, composeSub: []string{"compose"}}}, root
}
func TestDeleteChecksUndeployedReferencesBeforeDown(t *testing.T) {
	for _, kind := range []string{"volume", "network"} {
		t.Run(kind, func(t *testing.T) {
			r, root := fixtureRuntime(t)
			resource := `{"name":"shared"}`
			config := `{"services":{"app":{"image":"test:1"}},"volumes":{},"networks":{}}`
			if kind == "volume" {
				config = `{"services":{"app":{"image":"test:1"}},"volumes":{"data":` + resource + `},"networks":{}}`
				putFixture(t, filepath.Join(root, "volume.json"), `[{"Name":"shared","Labels":{"com.docker.compose.project":"test"}}]`)
			} else {
				config = `{"services":{"app":{"image":"test:1"}},"volumes":{},"networks":{"data":` + resource + `}}`
				putFixture(t, filepath.Join(root, "network.json"), `[{"Id":"nid","Name":"shared","Labels":{"com.docker.compose.project":"test"}}]`)
			}
			putFixture(t, filepath.Join(r.StackPath("test"), "resolved.json"), config)
			if err := os.Mkdir(r.StackPath("other"), 0755); err != nil {
				t.Fatal(err)
			}
			putFixture(t, filepath.Join(r.StackPath("other"), "compose.yaml"), "services: {}")
			putFixture(t, filepath.Join(r.StackPath("other"), "resolved.json"), config)
			_, err := r.CleanStackResources(context.Background(), "test")
			if err == nil || !strings.Contains(err.Error(), "其他栈 other") {
				t.Fatalf("err=%v", err)
			}
			commands, _ := os.ReadFile(filepath.Join(root, "commands"))
			if strings.Contains(string(commands), " down ") {
				t.Fatalf("阻止前执行了 down: %s", commands)
			}
		})
	}
}
func TestDeleteDownFailurePreservesDirectory(t *testing.T) {
	r, root := fixtureRuntime(t)
	putFixture(t, filepath.Join(root, "fail-down"), "")
	_, err := r.CleanStackResources(context.Background(), "test")
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("err=%v", err)
	}
	if _, e := os.Stat(r.StackPath("test")); e != nil {
		t.Fatal("失败时丢失了栈目录", e)
	}
}
func TestDeleteMultiTagImageAndFinalVerification(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "专属双标签清理", true: "镜像残留阻止目录删除"}[keep], func(t *testing.T) {
			r, root := fixtureRuntime(t)
			putFixture(t, filepath.Join(root, "image.json"), `[{"Id":"sha256:test","RepoTags":["test:1","test:2"],"Created":"2026-01-01T00:00:00Z"}]`)
			if keep {
				putFixture(t, filepath.Join(root, "keep-image"), "")
			}
			_, err := r.CleanStackResources(context.Background(), "test")
			if (err != nil) != keep {
				t.Fatalf("err=%v keep=%v", err, keep)
			}
			commands, _ := os.ReadFile(filepath.Join(root, "commands"))
			for _, tag := range []string{"test:1", "test:2"} {
				if !strings.Contains(string(commands), "image rm "+tag) {
					t.Fatalf("未删除标签 %s: %s", tag, commands)
				}
			}
		})
	}
}
func TestDeleteAnonymousOwnershipAndExternalRetention(t *testing.T) {
	for _, anonymous := range []bool{false, true} {
		t.Run(map[bool]string{false: "64位命名卷不能猜测归属", true: "按服务挂载识别匿名卷"}[anonymous], func(t *testing.T) {
			r, root := fixtureRuntime(t)
			vol := strings.Repeat("a", 64)
			mounts := []cleanupMount{}
			if anonymous {
				mounts = append(mounts, cleanupMount{Type: "volume", Target: "/data"})
			}
			putFixture(t, filepath.Join(r.StackPath("test"), "resolved.json"), jsonFixture(t, cleanupConfig{Services: map[string]cleanupService{"app": {Image: "test:1", Volumes: mounts}}, Volumes: map[string]cleanupResource{"ext": {Name: "external", External: true}}, Networks: map[string]cleanupResource{}}))
			putFixture(t, filepath.Join(root, "container.json"), `[{"Id":"cid","Name":"/test-app","Config":{"Image":"test:1","Labels":{"com.docker.compose.project":"test","com.docker.compose.service":"app"}},"Mounts":[{"Type":"volume","Name":"`+vol+`","Destination":"/data"}]}]`)
			putFixture(t, filepath.Join(root, "volume.json"), `[{"Name":"`+vol+`"},{"Name":"external"}]`)
			plan, err := r.PlanStackDelete(context.Background(), "test")
			if err != nil {
				t.Fatal(err)
			}
			if (len(plan.Blockers) > 0) == anonymous {
				t.Fatalf("错误归属: %+v", plan)
			}
			if anonymous && (len(plan.Volumes) != 1 || plan.Volumes[0] != vol) {
				t.Fatalf("缺少匿名卷: %+v", plan)
			}
			if !strings.Contains(strings.Join(plan.Retained, ","), "外部卷 external") {
				t.Fatalf("外部卷未交代: %+v", plan)
			}
		})
	}
}
func TestSaveKeepsComposeFilenameAndRejectsSymlinks(t *testing.T) {
	r := &Repository{stacksDir: t.TempDir()}
	ctx := context.Background()
	if err := os.Mkdir(r.StackPath("old"), 0755); err != nil {
		t.Fatal(err)
	}
	putFixture(t, filepath.Join(r.StackPath("old"), "docker-compose.yml"), "old")
	putFixture(t, filepath.Join(r.StackPath("old"), "data.txt"), "keep")
	if err := r.Save(ctx, &model.Stack{Name: "old", Yaml: "new"}, false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(r.StackPath("old"), "docker-compose.yml"))
	if string(b) != "new" {
		t.Fatal("旧文件未更新")
	}
	if _, err := os.Stat(filepath.Join(r.StackPath("old"), "compose.yaml")); !os.IsNotExist(err) {
		t.Fatal("产生多余 compose.yaml")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, r.StackPath("link")); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(ctx, &model.Stack{Name: "link", Yaml: "no"}, false); err == nil {
		t.Fatal("未拒绝符号链接目录")
	}
	if err := r.Delete(ctx, "link"); err == nil {
		t.Fatal("未拒绝删除符号链接目录")
	}
}

func TestDeleteCleansConfirmedOrphanContainer(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "清理 Compose 漏掉的孤立容器", true: "停止失败保留目录与错误明细"}[fail], func(t *testing.T) {
			r, root := fixtureRuntime(t)
			putFixture(t, filepath.Join(root, "orphan"), "")
			putFixture(t, filepath.Join(root, "container.json"), `[{"Id":"cid","Name":"/test-old","Config":{"Image":"test:1","Labels":{"com.docker.compose.project":"test","com.docker.compose.service":"old"}},"State":{"Status":"running"}}]`)
			if fail {
				putFixture(t, filepath.Join(root, "fail-stop"), "")
			}
			_, err := r.CleanStackResources(context.Background(), "test")
			if (err != nil) != fail {
				t.Fatalf("fail=%v err=%v", fail, err)
			}
			commands, _ := os.ReadFile(filepath.Join(root, "commands"))
			if !strings.Contains(string(commands), "stop cid") {
				t.Fatalf("未停止孤立容器: %s", commands)
			}
			if fail {
				if !strings.Contains(err.Error(), "orphan stop denied") || strings.Contains(string(commands), "rm cid") {
					t.Fatalf("错误明细或失败保护不正确: %v %s", err, commands)
				}
			} else if !strings.Contains(string(commands), "rm cid") {
				t.Fatalf("未删除孤立容器: %s", commands)
			}
		})
	}
}

// TestDeleteDigestImageReferences 识别未部署栈的 digest 引用，专属可删，共享必须保留。
func TestDeleteDigestImageReferences(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "专属 digest", true: "未部署共享 digest"}[shared], func(t *testing.T) {
			r, root := fixtureRuntime(t)
			digest := "registry.example/test@sha256:" + strings.Repeat("a", 64)
			putFixture(t, filepath.Join(root, "image.json"), jsonFixture(t, []map[string]interface{}{{"Id": "sha256:test", "RepoTags": []string{"test:1"}, "RepoDigests": []string{digest}}}))
			config := jsonFixture(t, cleanupConfig{Services: map[string]cleanupService{"app": {Image: digest}}})
			putFixture(t, filepath.Join(r.StackPath("test"), "resolved.json"), config)
			if shared {
				if err := os.Mkdir(r.StackPath("other"), 0755); err != nil {
					t.Fatal(err)
				}
				putFixture(t, filepath.Join(r.StackPath("other"), "compose.yaml"), "services: {}")
				putFixture(t, filepath.Join(r.StackPath("other"), "resolved.json"), config)
			}
			plan, err := r.PlanStackDelete(context.Background(), "test")
			if err != nil {
				t.Fatal(err)
			}
			if shared {
				if len(plan.Images) != 0 || !strings.Contains(strings.Join(plan.Retained, ","), "共享镜像") {
					t.Fatalf("shared digest not retained: %+v", plan)
				}
			} else if len(plan.Images) != 1 || plan.Images[0] != "sha256:test" {
				t.Fatalf("digest image missing: %+v", plan)
			}
		})
	}
}

// TestDeleteImplicitLatestReferences 防止省略标签的共享引用漏检，同时保留非 latest 标签的独立性。
func TestDeleteImplicitLatestReferences(t *testing.T) {
	for _, tc := range []struct {
		name, own, tag, other string
		shared                bool
	}{
		{"专属省略标签", "example/app", "example/app:latest", "", false},
		{"共享省略标签", "example/app:latest", "example/app:latest", "example/app", true},
		{"注册表端口不是镜像标签", "registry.example:5000/app", "registry.example:5000/app:latest", "registry.example:5000/app", true},
		{"非 latest 标签不混同", "example/app:1", "example/app:1", "example/app", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, root := fixtureRuntime(t)
			putFixture(t, filepath.Join(root, "image.json"), jsonFixture(t, []map[string]interface{}{{"Id": "sha256:test", "RepoTags": []string{tc.tag}}}))
			putFixture(t, filepath.Join(r.StackPath("test"), "resolved.json"), jsonFixture(t, cleanupConfig{Services: map[string]cleanupService{"app": {Image: tc.own}}}))
			if tc.other != "" {
				if err := os.Mkdir(r.StackPath("other"), 0755); err != nil {
					t.Fatal(err)
				}
				putFixture(t, filepath.Join(r.StackPath("other"), "compose.yaml"), "services: {}")
				putFixture(t, filepath.Join(r.StackPath("other"), "resolved.json"), jsonFixture(t, cleanupConfig{Services: map[string]cleanupService{"app": {Image: tc.other}}}))
			}
			plan, err := r.PlanStackDelete(context.Background(), "test")
			if err != nil {
				t.Fatal(err)
			}
			if tc.shared {
				if len(plan.Images) != 0 || !strings.Contains(strings.Join(plan.Retained, ","), "共享镜像") {
					t.Fatalf("共享镜像未保留: %+v", plan)
				}
			} else if len(plan.Images) != 1 || plan.Images[0] != "sha256:test" {
				t.Fatalf("专属镜像未列入清理范围: %+v", plan)
			}
		})
	}
}

// TestDeleteBlocksForeignContainerBind 覆盖目录外项目和非 Compose 容器对栈内数据的实际引用。
func TestDeleteBlocksForeignContainerBind(t *testing.T) {
	for _, tc := range []struct {
		name, project, state string
		inside, symlink      bool
	}{
		{name: "运行中独立容器", state: "running", inside: true},
		{name: "停止的独立容器", state: "exited", inside: true},
		{name: "目录外项目", project: "outside", state: "running", inside: true},
		{name: "符号链接引用", state: "exited", inside: true, symlink: true},
		{name: "无关栈外挂载", state: "running"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, root := fixtureRuntime(t)
			source := t.TempDir()
			if tc.inside {
				source = filepath.Join(r.StackPath("test"), "data")
				if err := os.Mkdir(source, 0o755); err != nil {
					t.Fatal(err)
				}
				putFixture(t, filepath.Join(source, "keep.txt"), "shared data")
			}
			if tc.symlink {
				link := filepath.Join(t.TempDir(), "shared")
				if err := os.Symlink(source, link); err != nil {
					t.Fatal(err)
				}
				source = link
			}
			c := inspectedContainer{ID: "foreign", Name: "/outside-reader"}
			c.Config.Labels = map[string]string{"com.docker.compose.project": tc.project}
			c.State.Status = tc.state
			data := jsonFixture(t, c)
			var row map[string]interface{}
			if err := json.Unmarshal([]byte(data), &row); err != nil {
				t.Fatal(err)
			}
			row["Mounts"] = []map[string]string{{"Type": "bind", "Source": source, "Destination": "/shared"}}
			putFixture(t, filepath.Join(root, "container.json"), jsonFixture(t, []interface{}{row}))
			_, err := r.CleanStackResources(context.Background(), "test")
			commands, readErr := os.ReadFile(filepath.Join(root, "commands"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if tc.inside {
				if err == nil || !strings.Contains(err.Error(), "foreign") || !strings.Contains(err.Error(), source) {
					t.Errorf("实际共享引用应阻止删除并说明容器与来源：%v", err)
				}
				if strings.Contains(string(commands), " down ") {
					t.Errorf("共享数据核验失败前执行了 down：%s", commands)
				}
				data, e := os.ReadFile(filepath.Join(source, "keep.txt"))
				if e != nil || string(data) != "shared data" {
					t.Fatalf("共享数据丢失：%q, %v", data, e)
				}
			} else if err != nil {
				t.Fatalf("无关栈外数据不应阻止目标清理：%v", err)
			}
		})
	}
}
