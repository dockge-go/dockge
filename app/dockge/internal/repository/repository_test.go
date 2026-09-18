package repository

import (
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

// TestInitBucketsCreatesOnlyKnownBuckets 守护 bucket 白名单：
// 只创建 users/settings，绝不创建历史遗留的 agents（多主机能力已移除）。
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
		for _, name := range []string{bucketUsers, bucketSettings} {
			if tx.Bucket([]byte(name)) == nil {
				t.Errorf("bucket %q must exist", name)
			}
		}
		if tx.Bucket([]byte("agents")) != nil {
			t.Error("agents bucket must not be created")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
