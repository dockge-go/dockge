package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dockge/internal/model"
	"dockge/internal/repository"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
	"go.etcd.io/bbolt"
)

func snapshotService(t *testing.T) (*stackService, string) {
	t.Helper()
	root := t.TempDir()
	calls := filepath.Join(root, "calls")
	cli := filepath.Join(root, "runtime")
	stacks := filepath.Join(root, "stacks")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
if [ "$*" != 'compose ls --all --format json' ]; then echo "unexpected command: $*" >&2; exit 1; fi
printf '%%s\n' '[{"Name":"managed","Status":"running(1)","ConfigFiles":"%s"},{"Name":"dockge","Status":"running(1)"}]'
`, calls, filepath.Join(stacks, "managed", "compose.yaml"))
	if err := os.WriteFile(cli, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := bbolt.Open(filepath.Join(root, "test.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg := viper.New()
	cfg.Set("security.jwt.key", "snapshot-test-key")
	cfg.Set("stacks_dir", stacks)
	cfg.Set("container.cli", cli)
	i := do.New()
	do.ProvideValue(i, cfg)
	do.ProvideValue(i, db)
	repo, err := repository.New(i)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"managed", "broken"} {
		if err := repo.Save(context.Background(), &model.Stack{Name: name, Yaml: "services: {}\n", Env: "A=1\n"}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(stacks, "broken", ".env")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(stacks, "broken", ".env"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &stackService{repo: repo}, calls
}

func TestStackSnapshotDoesNotWaitForDeploymentOrLoadDetails(t *testing.T) {
	s, calls := snapshotService(t)
	unlock, err := lockStack(context.Background(), "managed")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rows, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("必须仅返回 stacks 目录内栈: %+v", rows)
	}
	for _, row := range rows {
		switch row.Name {
		case "managed":
			if row.Status != int(model.StatusRunning) || row.Yaml != "services: {}\n" || row.Env != "A=1\n" || row.LoadError != "" {
				t.Fatalf("真实状态和配置: %+v", row)
			}
		case "broken":
			if !strings.Contains(row.LoadError, ".env") || !strings.Contains(row.LoadError, "directory") {
				t.Fatalf("文件读取失败须保留明细: %+v", row)
			}
		default:
			t.Fatalf("不应包含面板自身: %+v", row)
		}
	}
	raw, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "compose ls --all --format json\n" {
		t.Fatalf("概览应仅一次列表查询，无逐栈 ps/inspect: %s", raw)
	}
}

func TestStackSnapshotCanceled(t *testing.T) {
	s, _ := snapshotService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Snapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消快照: %v", err)
	}
}
