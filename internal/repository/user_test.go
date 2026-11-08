package repository

import (
	"context"
	"encoding/json"
	"errors"
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

// signaledUserContext 在第一次检查时通知测试，检查结果取通知前的值，避免取消竞态遮蔽写锁等待路径。
type signaledUserContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *signaledUserContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return err
}

// TestUserWritesCanceled 取消的初始化、改密和认证设置写入不得落库，包括等待写锁后取消。
func TestUserWritesCanceled(t *testing.T) {
	for _, operation := range []string{"create", "password", "setting"} {
		for _, waiting := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/waiting=%v", operation, waiting), func(t *testing.T) {
				db, err := bbolt.Open(t.TempDir()+"/test.db", 0o600, nil)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Close() })
				if err := initBuckets(db); err != nil {
					t.Fatal(err)
				}
				r := &Repository{db: db}
				user := &model.DockgeUser{Username: "ops", Password: "original", Role: model.RoleAdmin, Active: true}
				if operation == "password" {
					if err := r.CreateUser(context.Background(), user); err != nil {
						t.Fatal(err)
					}
				}
				write := func(ctx context.Context) error {
					switch operation {
					case "create":
						return r.CreateUser(ctx, user)
					case "password":
						return r.UpdatePassword(ctx, user.ID, "changed")
					default:
						return r.SetSetting(ctx, "disableAuth", "true", "security")
					}
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if waiting {
					tx, txErr := db.Begin(true)
					if txErr != nil {
						t.Fatal(txErr)
					}
					observed := &signaledUserContext{Context: ctx, checked: make(chan struct{})}
					done := make(chan error, 1)
					go func() { done <- write(observed) }()
					select {
					case <-observed.checked:
					case <-time.After(time.Second):
						t.Error("写入入口未检查请求取消")
					}
					cancel()
					if err := tx.Rollback(); err != nil {
						t.Fatal(err)
					}
					err = <-done
				} else {
					cancel()
					err = write(ctx)
				}
				if !errors.Is(err, context.Canceled) {
					t.Errorf("已取消写入应返回 context.Canceled，实际 %v", err)
				}
				switch operation {
				case "create":
					if count, e := r.CountUsers(context.Background()); e != nil || count != 0 {
						t.Errorf("取消初始化仍创建了账号：%d, %v", count, e)
					}
					if user.ID != 0 || !user.CreatedAt.IsZero() || !user.UpdatedAt.IsZero() {
						t.Error("取消初始化改写了调用方的账号数据")
					}
				case "password":
					if got, e := r.SingleUser(context.Background()); e != nil || got.Password != "original" {
						t.Errorf("取消改密仍改变了密码：%q, %v", got.Password, e)
					}
				case "setting":
					if _, e := r.GetSetting(context.Background(), "disableAuth"); !errors.Is(e, ErrNotFound) {
						t.Errorf("取消设置仍落库：%v", e)
					}
				}
			})
		}
	}
}
