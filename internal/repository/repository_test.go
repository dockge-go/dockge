package repository

import (
	"path/filepath"
	"testing"

	"dockge/pkg/config"

	"go.etcd.io/bbolt"
)

// TestInitBucketsCreatesOnlyKnownBuckets 守护 bucket 白名单：
// 只创建 users/settings，不产生多余 bucket（单机版：无 agents）。
func TestInitBucketsCreatesOnlyKnownBuckets(t *testing.T) {
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "dockge.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := initBuckets(db); err != nil {
		t.Fatal(err)
	}

	err = db.View(func(tx *bbolt.Tx) error {
		want := map[string]bool{bucketUsers: true, bucketSettings: true}
		for name := range want {
			if tx.Bucket([]byte(name)) == nil {
				t.Errorf("bucket %q must exist", name)
			}
		}
		count := 0
		if err := tx.ForEach(func(name []byte, _ *bbolt.Bucket) error {
			count++
			if !want[string(name)] {
				t.Errorf("unexpected bucket %q", name)
			}
			return nil
		}); err != nil {
			return err
		}
		if count != len(want) {
			t.Errorf("bucket count = %d, want %d", count, len(want))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestStacksDirFromConf 守护栈目录解析：环境变量 DOCKGE_STACKS_DIR 覆盖默认值
// （上游同名变量，从上游迁移的编排不改动即可继续工作）。
func TestStacksDirFromConf(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"缺省", "", "storage/stacks"},
		{"环境变量覆盖默认值", "/opt/stacks", "/opt/stacks"},
		{"环境变量去空白", "  /opt/stacks  ", "/opt/stacks"},
		{"空白环境变量视为未设", "   ", "storage/stacks"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOCKGE_STACKS_DIR", tc.env)
			conf, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if got := StacksDirFromConf(conf); got != tc.want {
				t.Errorf("StacksDirFromConf() = %q, want %q", got, tc.want)
			}
		})
	}
}
