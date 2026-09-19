package repository

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
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

// TestStacksDirFromConf 守护栈目录优先级：上游同名环境变量 DOCKGE_STACKS_DIR
// 优先于配置文件（否则按上游编排迁移的用户会静默用错目录、看不到自己的栈）。
func TestStacksDirFromConf(t *testing.T) {
	cases := []struct {
		name string
		env  string
		conf string
		want string
	}{
		{"缺省", "", "", "storage/stacks"},
		{"配置文件优先于缺省", "", "/srv/conf-stacks", "/srv/conf-stacks"},
		{"环境变量优先于配置", "/opt/stacks", "/srv/conf-stacks", "/opt/stacks"},
		{"环境变量去空白", "  /opt/stacks  ", "", "/opt/stacks"},
		{"空白环境变量视为未设", "   ", "/srv/conf-stacks", "/srv/conf-stacks"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOCKGE_STACKS_DIR", tc.env)
			conf := viper.New()
			if tc.conf != "" {
				conf.Set("dockge.stacks_dir", tc.conf)
			}
			if got := StacksDirFromConf(conf); got != tc.want {
				t.Errorf("StacksDirFromConf() = %q, want %q", got, tc.want)
			}
		})
	}
}
