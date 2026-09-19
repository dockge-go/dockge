package repository

import (
	"context"
	"testing"

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

// TestUserRoundTrip 覆盖创建、按名/按 ID 读取、计数与改密的往返。
func TestUserRoundTrip(t *testing.T) {
	repo, uid := newTestUserDB(t)
	ctx := context.Background()

	if n, err := repo.CountUsers(ctx); err != nil || n != 1 {
		t.Fatalf("CountUsers = (%d, %v), want (1, nil)", n, err)
	}
	byName, err := repo.GetUserByUsername(ctx, "ops")
	if err != nil || byName.ID != uid {
		t.Fatalf("GetUserByUsername = (%d, %v), want (%d, nil)", byName.ID, err, uid)
	}
	if err := repo.UpdatePassword(ctx, uid, "new-hash"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	got, err := repo.GetUser(ctx, uid)
	if err != nil || got.Password != "new-hash" {
		t.Fatalf("GetUser after update = (%q, %v), want (new-hash, nil)", got.Password, err)
	}
	users, err := repo.ListUsers(ctx)
	if err != nil || len(users) != 1 || users[0].ID != uid {
		t.Fatalf("ListUsers = (%d users, %v), want (1, nil)", len(users), err)
	}
	if _, err := repo.GetUser(ctx, uid+1); err != ErrNotFound {
		t.Errorf("GetUser missing = %v, want ErrNotFound", err)
	}
}
