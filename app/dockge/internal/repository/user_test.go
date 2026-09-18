package repository

import (
	"context"
	"testing"

	"go.etcd.io/bbolt"

	"dockge/app/dockge/internal/model"
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
	seed := &model.DockgeUser{Username: "ops", Password: "x", Role: model.RoleMember, Active: true}
	if err := repo.CreateUser(context.Background(), seed); err != nil {
		t.Fatal(err)
	}
	return repo, seed.ID
}

// TestSetUserActiveAndRole 覆盖启用/停用与角色变更的往返。
func TestSetUserActiveAndRole(t *testing.T) {
	repo, uid := newTestUserDB(t)
	ctx := context.Background()

	if err := repo.SetUserActive(ctx, uid, false); err != nil {
		t.Fatalf("SetUserActive: %v", err)
	}
	got, err := repo.GetUser(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Active {
		t.Error("user should be deactivated")
	}

	if err := repo.SetUserRole(ctx, uid, model.RoleAdmin); err != nil {
		t.Fatalf("SetUserRole: %v", err)
	}
	got, err = repo.GetUser(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != model.RoleAdmin {
		t.Errorf("role = %q, want admin", got.Role)
	}
}

// TestDeleteUser 覆盖删除存在与不存在的账号。
func TestDeleteUser(t *testing.T) {
	repo, uid := newTestUserDB(t)
	ctx := context.Background()

	if err := repo.DeleteUser(ctx, uid); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := repo.GetUser(ctx, uid); err != ErrNotFound {
		t.Errorf("GetUser after delete = %v, want ErrNotFound", err)
	}
	if err := repo.DeleteUser(ctx, uid); err != ErrNotFound {
		t.Errorf("DeleteUser twice = %v, want ErrNotFound", err)
	}
}
