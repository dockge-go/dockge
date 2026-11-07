package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiskSpaceTruth(t *testing.T) {
	for _, tc := range []struct {
		input   string
		count   int
		reclaim float64
		bad     bool
	}{
		{`{"Type":"Images","Size":"2.359GB","Reclaimable":"0B (0%)","TotalCount":"3"}`, 3, 0, false},
		{`{"Type":"Build Cache","Size":"21.54GB","Reclaimable":"20.6GB","TotalCount":248}`, 248, 20600000000, false},
		{`{"Type":"Images","Size":"N/A","Reclaimable":"N/A","TotalCount":"3"}`, 3, 0, false},
		{`{"Type":"Images","TotalCount":"garbage"}`, 0, 0, true},
	} {
		rows, err := parseDiskRows(tc.input)
		if (err != nil) != tc.bad {
			t.Fatalf("%s: %v", tc.input, err)
		}
		if !tc.bad && (len(rows) != 1 || rows[0].Count != tc.count || rows[0].ReclaimBytes != tc.reclaim) {
			t.Fatalf("%s: %+v", tc.input, rows)
		}
		if !tc.bad && rows[0].ReclaimKnown != (tc.reclaim > 0 || strings.Contains(tc.input, "0B")) {
			t.Fatalf("未知空间不应伪装为已知: %+v", rows)
		}
	}
}

func TestRunningContainerDeleteStopsFirst(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "先停止再删除", true: "停止失败保留容器"}[fail], func(t *testing.T) {
			r, root := fixtureRuntime(t)
			putFixture(t, filepath.Join(root, "container.json"), `[{"Id":"cid","Name":"/test-app","State":{"Status":"running"}}]`)
			if fail {
				putFixture(t, filepath.Join(root, "fail-stop"), "")
			}
			_, err := r.ResourceAction(context.Background(), "containers", "cid", "delete", "")
			if (err != nil) != fail {
				t.Fatalf("err=%v", err)
			}
			commands, _ := os.ReadFile(filepath.Join(root, "commands"))
			s := string(commands)
			stop, remove := strings.Index(s, "stop cid"), strings.Index(s, "rm -v cid")
			if stop < 0 || fail && remove >= 0 || !fail && remove <= stop {
				t.Fatalf("不安全的删除次序: %s", s)
			}
		})
	}
}

func TestImageDeleteVerifiesRemaining(t *testing.T) {
	r, root := fixtureRuntime(t)
	putFixture(t, filepath.Join(root, "image.json"), `[{"Id":"sha256:test","RepoTags":["test:1","test:2"]}]`)
	putFixture(t, filepath.Join(root, "keep-image"), "")
	if _, err := r.ResourceAction(context.Background(), "images", "sha256:test", "delete", ""); err == nil {
		t.Fatal("镜像残留却报告删除成功")
	}
}

func TestContainerDeleteStateSafety(t *testing.T) {
	for _, tc := range []struct {
		state, failure string
		wantStop, bad  bool
	}{
		{"restarting", "", true, false}, {"paused", "", true, false},
		{"paused", "fail-unpause", false, true}, {"created", "", false, false}, {"dead", "", false, false},
	} {
		t.Run(tc.state+tc.failure, func(t *testing.T) {
			r, root := fixtureRuntime(t)
			putFixture(t, filepath.Join(root, "container.json"), fmt.Sprintf(`[{"Id":"cid","Name":"/test-app","State":{"Status":%q}}]`, tc.state))
			if tc.failure != "" {
				putFixture(t, filepath.Join(root, tc.failure), "")
			}
			_, err := r.ResourceAction(context.Background(), "containers", "cid", "delete", "")
			if (err != nil) != tc.bad {
				t.Fatalf("err=%v", err)
			}
			cmds, _ := os.ReadFile(filepath.Join(root, "commands"))
			text := string(cmds)
			if strings.Contains(text, "stop cid") != tc.wantStop || strings.Contains(text, "rm -v cid") == tc.bad {
				t.Fatalf("删除次序不安全: %s", text)
			}
			if tc.state == "paused" && (!strings.Contains(text, "unpause cid") || tc.wantStop && strings.Index(text, "unpause cid") > strings.Index(text, "stop cid")) {
				t.Fatalf("暂停容器必须先恢复: %s", text)
			}
		})
	}
}

func TestBuildCacheCleanupScope(t *testing.T) {
	r, root := fixtureRuntime(t)
	out, err := r.ResourceAction(context.Background(), "buildCache", "unused", "delete", "")
	if err != nil || !strings.Contains(out, "Total reclaimed space") {
		t.Fatalf("out=%s err=%v", out, err)
	}
	cmds, _ := os.ReadFile(filepath.Join(root, "commands"))
	if string(cmds) != "builder prune --all --force\n" {
		t.Fatalf("缓存清理不能扩大范围: %s", cmds)
	}
	for _, id := range []string{"all", "images", ""} {
		if _, err := r.ResourceAction(context.Background(), "buildCache", id, "delete", ""); err == nil {
			t.Fatalf("拒绝未经约定的缓存目标 %q", id)
		}
	}
}
