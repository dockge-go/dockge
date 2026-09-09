package server

import (
	"context"
	"encoding/json"
	"fmt"

	"dockge/app/dockge/internal/model"
	"dockge/app/dockge/internal/repository"
	"dockge/pkg/log"

	"github.com/samber/do/v2"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

// MigrateServer 是一次性迁移进程：初始化 bbolt bucket、种子默认账号，完成后退出。
type MigrateServer struct {
	db   *bbolt.DB
	re   *repository.Repository
	log  *log.Logger
	done chan struct{} // 迁移完成信号；main 据此正常退出（避免在服务层 os.Exit）
}

// Done 返回迁移完成信号通道。
func (m *MigrateServer) Done() <-chan struct{} { return m.done }

// NewMigrateServer 构造一次性迁移服务，由注入容器调用。

func NewMigrateServer(i do.Injector) (*MigrateServer, error) {
	return &MigrateServer{
		db:   do.MustInvoke[*bbolt.DB](i),
		re:   do.MustInvoke[*repository.Repository](i),
		log:  do.MustInvoke[*log.Logger](i),
		done: make(chan struct{}),
	}, nil
}

// Start 执行 bucket 初始化与默认数据种子，完成后关闭 Done 通道。

func (m *MigrateServer) Start(ctx context.Context) error {
	if err := m.initBuckets(); err != nil {
		return err
	}
	if err := m.seedAdmin(); err != nil {
		return err
	}
	if err := m.re.EnsureStacksDir(); err != nil {
		return err
	}
	close(m.done) // 通知 main 正常退出
	m.log.Info().Msg("dockge migrate success (bbolt)")
	return nil
}

// Stop 无需清理（迁移是一次性动作）。

func (m *MigrateServer) Stop(ctx context.Context) error { return nil }

func (m *MigrateServer) initBuckets() error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		for _, name := range []string{"users", "settings"} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bucket %q: %w", name, err)
			}
		}
		return nil
	})
}

func (m *MigrateServer) seedAdmin() error {
	hashed, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash seed password: %w", err)
	}
	user := &model.DockgeUser{
		Username: "admin",
		Nickname: "管理员",
		Password: string(hashed),
		Role:     model.RoleAdmin,
		Active:   true,
	}
	data, _ := json.Marshal(user)
	return m.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		// 用 username 作 key，与 CreateUser 一致
		if b.Get([]byte("admin")) != nil {
			return nil // 已存在
		}
		id, _ := b.NextSequence()
		user.ID = uint(id)
		data, _ = json.Marshal(user)
		return b.Put([]byte("admin"), data)
	})
}
