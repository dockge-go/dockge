package service

import (
	"context"
	"testing"

	"github.com/samber/do/v2"
	"github.com/spf13/viper"

	"dockge/internal/model"
	"dockge/internal/repository"
	"dockge/pkg/hash"
)

// hashBcryptForTest 测试用 bcrypt 包装（真实哈希以验证 Active 分支可达）。
func hashBcryptForTest(pw string) string {
	h, _ := hash.BcryptHash(pw)
	return h
}

// newTestAuthRepo 在临时文件上建库并种入 admin 用户，供 service 层测试直接构造 authService。
func newTestAuthRepo(t *testing.T) (*repository.Repository, uint) {
	t.Helper()
	conf := viper.New()
	conf.Set("data.db.user.dsn", t.TempDir()+"/test.db")
	injector := do.New()
	do.ProvideValue(injector, conf)
	db, err := repository.NewBBolt(injector)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	do.ProvideValue(injector, db)
	repo, err := repository.New(injector)
	if err != nil {
		t.Fatal(err)
	}
	seed := &model.DockgeUser{Username: "admin", Password: "x", Role: model.RoleAdmin, Active: true}
	if err := repo.CreateUser(context.Background(), seed); err != nil {
		t.Fatal(err)
	}
	return repo, seed.ID
}
