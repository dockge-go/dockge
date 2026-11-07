package repository

import (
	"context"
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
