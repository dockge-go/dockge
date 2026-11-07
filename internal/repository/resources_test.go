package repository

import (
	"context"
	v1 "dockge/api/v1"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeRuntimeRows(t *testing.T) {
	type row struct{ Name string }
	for _, tc := range []struct {
		name, input string
		count       int
		bad         bool
	}{
		{"空输出", " \n", 0, false}, {"数组", `[{"Name":"a"},{"Name":"b"}]`, 2, false},
		{"完整多行对象", "{\n  \"Name\": \"a\"\n}\n", 1, false}, {"连续对象", "{\"Name\":\"a\"}\n{\"Name\":\"b\"}", 2, false},
		{"损坏 JSON", `{"Name":`, 0, true}, {"尾部垃圾", `{"Name":"a"}invalid`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := decodeRows[row](tc.input)
			if (err != nil) != tc.bad || err == nil && len(rows) != tc.count {
				t.Fatalf("rows=%v err=%v", rows, err)
			}
		})
	}
}
func TestRuntimeBytesAndCommand(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want float64
		bad  bool
	}{{"1.5MiB", 1572864, false}, {"2 kB", 2000, false}, {"0B", 0, false}, {"NaNB", 0, true}, {"-2GB", 0, true}, {"2foo", 0, true}} {
		n, err := parseBytes(tc.s)
		if (err != nil) != tc.bad || !tc.bad && n != tc.want {
			t.Errorf("parseBytes(%q)=%v,%v", tc.s, n, err)
		}
	}
	args, err := splitCommand(`sh -c 'echo hello; echo $HOME' ""`)
	if err != nil || !reflect.DeepEqual(args, []string{"sh", "-c", "echo hello; echo $HOME", ""}) {
		t.Fatalf("args=%q err=%v", args, err)
	}
	if _, err := splitCommand(`echo "oops`); err == nil {
		t.Fatal("未拒绝不闭合引号")
	}
}

func TestImageDeleteMultiTagAndContainerProtection(t *testing.T) {
	for _, used := range []bool{false, true} {
		t.Run(map[bool]string{false: "非强制清理全部标签", true: "占用镜像不删除任何标签"}[used], func(t *testing.T) {
			r, root := fixtureRuntime(t)
			putFixture(t, filepath.Join(root, "image.json"), `[{"Id":"sha256:test","RepoTags":["test:1","test:2"]}]`)
			if used {
				putFixture(t, filepath.Join(root, "container.json"), `[{"Id":"cid","Image":"sha256:test","Config":{"Image":"test:1"}}]`)
			}
			_, err := r.ResourceAction(context.Background(), "images", "sha256:test", "delete", "")
			if (err != nil) != used {
				t.Fatalf("used=%v err=%v", used, err)
			}
			commands, _ := os.ReadFile(filepath.Join(root, "commands"))
			if used && strings.Contains(string(commands), "image rm") {
				t.Fatalf("删除了占用镜像: %s", commands)
			}
			if !used && (!strings.Contains(string(commands), "image rm test:1") || !strings.Contains(string(commands), "image rm test:2")) {
				t.Fatalf("标签清理不完整: %s", commands)
			}
		})
	}
}

func TestStoppedContainerReferencesBlockNetworkAndVolumeDelete(t *testing.T) {
	for _, kind := range []string{"networks", "volumes"} {
		t.Run(kind, func(t *testing.T) {
			r, root := fixtureRuntime(t)
			putFixture(t, filepath.Join(root, "container.json"), `[{"Id":"cid","Name":"/stopped","State":{"Status":"exited"},"Mounts":[{"Type":"volume","Name":"data"}],"NetworkSettings":{"Networks":{"private":{}}}}]`)
			putFixture(t, filepath.Join(root, "network.json"), `[{"Id":"nid","Name":"private","Containers":{}}]`)
			putFixture(t, filepath.Join(root, "volume.json"), `[{"Name":"data"}]`)
			id := "nid"
			if kind == "volumes" {
				id = "data"
			}
			_, err := r.ResourceAction(context.Background(), kind, id, "delete", "")
			if err == nil || !strings.Contains(err.Error(), "停止") {
				t.Fatalf("err=%v", err)
			}
			commands, _ := os.ReadFile(filepath.Join(root, "commands"))
			if strings.Contains(string(commands), "network rm") || strings.Contains(string(commands), "volume rm") {
				t.Fatalf("删除了占用资源: %s", commands)
			}
		})
	}
}

func TestBuiltinNetworkByIDAndDuplicateVolume(t *testing.T) {
	r, root := fixtureRuntime(t)
	putFixture(t, filepath.Join(root, "network.json"), `[{"Id":"nid","Name":"bridge"}]`)
	if _, err := r.ResourceAction(context.Background(), "networks", "nid", "delete", ""); err == nil {
		t.Fatal("通过 ID 绕过了系统网络保护")
	}
	putFixture(t, filepath.Join(root, "volume.json"), `[{"Name":"existing"}]`)
	if _, err := r.ResourceCreate(context.Background(), "volumes", v1.ResourceCreateRequest{Name: "existing"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复新增: %v", err)
	}
}

func TestStartPausedContainerUsesUnpause(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "暂停进程恢复执行", true: "恢复失败透出运行时原因"}[failed], func(t *testing.T) {
			r, root := fixtureRuntime(t)
			putFixture(t, filepath.Join(root, "container.json"), `[{"Id":"cid","Name":"/paused","State":{"Status":"paused"}}]`)
			if failed {
				putFixture(t, filepath.Join(root, "fail-unpause"), "")
			}
			_, err := r.ResourceAction(context.Background(), "containers", "cid", "start", "")
			if (err != nil) != failed || failed && !strings.Contains(err.Error(), "unpause denied") {
				t.Fatalf("恢复暂停容器结果不正确: %v", err)
			}
			commands, err := os.ReadFile(filepath.Join(root, "commands"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(commands), "unpause cid\n") || strings.Contains(string(commands), "start cid\n") {
				t.Fatalf("暂停容器应恢复而非重复启动: %s", commands)
			}
		})
	}
}
