package server

import (
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

func TestMigrateInitBucketsDoesNotCreateAgentBucket(t *testing.T) {
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "dockge.db"), 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	migrate := &MigrateServer{db: db}
	if err := migrate.initBuckets(); err != nil {
		t.Fatal(err)
	}

	err = db.View(func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte("agents")) != nil {
			t.Fatal("agents bucket must not be created")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
