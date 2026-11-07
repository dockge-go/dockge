package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.etcd.io/bbolt"

	"dockge/internal/model"
)

// newTestUserDB 在临时文件上建库并种入一个用户，返回仓储与其 ID。
func newTestUserDB(t *testing.T) (*Repository, uint) {
	t.Helper()
	db, err := bbolt.Open(t.TempDir()+"/test.db", 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := initBuckets(db); err != nil {
		t.Fatal(err)
	}
	repo := &Repository{db: db}
	seed := &model.DockgeUser{Username: "ops", Password: "x", Role: model.RoleAdmin, Active: true}
	if err := repo.CreateUser(context.Background(), seed); err != nil {
		t.Fatal(err)
	}
	return repo, seed.ID
}

// TestUserRoundTrip 覆盖初始化、唯一账号读取、计数与改密的往返。
func TestUserRoundTrip(t *testing.T) {
	repo, uid := newTestUserDB(t)
	ctx := context.Background()

	if n, err := repo.CountUsers(ctx); err != nil || n != 1 {
		t.Fatalf("CountUsers = (%d, %v), want (1, nil)", n, err)
	}
	if err := repo.UpdatePassword(ctx, uid, "new-hash"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	got, err := repo.SingleUser(ctx)
	if err != nil || got.Password != "new-hash" {
		t.Fatalf("GetUser after update = (%q, %v), want (new-hash, nil)", got.Password, err)
	}
	user, err := repo.SingleUser(ctx)
	if err != nil || user.ID != uid {
		t.Fatalf("SingleUser = %+v, %v", user, err)
	}
	if err := repo.UpdatePassword(ctx, uid+1, "invalid"); err != ErrNotFound {
		t.Errorf("UpdatePassword missing = %v, want ErrNotFound", err)
	}
}

// TestSingleUserAtomicInitialization 并发初始化只能有一个成功，不依赖服务层先查再写。
func TestSingleUserAtomicInitialization(t *testing.T) {
	repo, _ := newTestUserDB(t)
	if err := repo.db.Update(func(tx *bbolt.Tx) error { return tx.DeleteBucket([]byte(bucketUsers)) }); err != nil {
		t.Fatal(err)
	}
	if err := initBuckets(repo.db); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- repo.CreateUser(context.Background(), &model.DockgeUser{Username: fmt.Sprintf("user%d", i), Role: model.RoleAdmin, Active: true})
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if err != ErrConflict {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("successes=%d", successes)
	}
	if n, err := repo.CountUsers(context.Background()); err != nil || n != 1 {
		t.Fatalf("count=%d error=%v", n, err)
	}
}

// TestSingleUserLegacySelection 历史多账号只选择原始管理员，不静默破坏旧库。
func TestSingleUserLegacySelection(t *testing.T) {
	repo, uid := newTestUserDB(t)
	if err := repo.db.Update(func(tx *bbolt.Tx) error {
		for _, u := range []model.DockgeUser{{ID: 0, Username: "member", Role: "member", Active: true}, {ID: uid + 1, Username: "extra", Role: model.RoleAdmin, Active: true}} {
			data, _ := json.Marshal(u)
			if err := tx.Bucket([]byte(bucketUsers)).Put([]byte(u.Username), data); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	u, err := repo.SingleUser(context.Background())
	if err != nil || u.ID != uid {
		t.Fatalf("primary=%+v error=%v", u, err)
	}
	if n, _ := repo.CountUsers(context.Background()); n != 3 {
		t.Fatal("silently removed legacy accounts")
	}
}

// TestUserTimestamps 确认新账号时间戳落库，导入记录已有时间不被覆盖。
func TestUserTimestamps(t *testing.T) {
	r, uid := newTestUserDB(t)
	ctx := context.Background()
	u, err := r.SingleUser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if u.CreatedAt.IsZero() || u.UpdatedAt.IsZero() || time.Since(u.CreatedAt) > time.Minute {
		t.Fatalf("missing real timestamps: %+v", u)
	}
	created := u.CreatedAt
	if err := r.UpdatePassword(ctx, uid, "updated"); err != nil {
		t.Fatal(err)
	}
	u, err = r.SingleUser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !u.CreatedAt.Equal(created) {
		t.Fatal("password update changed creation date")
	}
}
