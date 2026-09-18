package server

import (
	"context"
	"errors"
	"fmt"

	"dockge/app/dockge/internal/model"
	"dockge/app/dockge/internal/repository"
	"dockge/pkg/log"

	"github.com/samber/do/v2"
	"golang.org/x/crypto/bcrypt"
)

// MigrateServer 是一次性迁移进程：初始化 bbolt bucket、种子默认账号，完成后退出。
// bucket 初始化由 NewBBolt 在 DI 解析 *bbolt.DB 时幂等完成，这里不再重复。
type MigrateServer struct {
	re   *repository.Repository
	log  *log.Logger
	done chan struct{} // 迁移完成信号；main 据此正常退出（避免在服务层 os.Exit）
}

// Done 返回迁移完成信号通道。
func (m *MigrateServer) Done() <-chan struct{} { return m.done }

// NewMigrateServer 构造一次性迁移服务，由注入容器调用。
func NewMigrateServer(i do.Injector) (*MigrateServer, error) {
	return &MigrateServer{
		re:   do.MustInvoke[*repository.Repository](i),
		log:  do.MustInvoke[*log.Logger](i),
		done: make(chan struct{}),
	}, nil
}

// Start 执行默认数据种子，完成后关闭 Done 通道。
func (m *MigrateServer) Start(ctx context.Context) error {
	if err := m.seedAdmin(ctx); err != nil {
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

// seedAdmin 种子默认管理员（admin/123456，仅首次）；已存在时幂等跳过。
func (m *MigrateServer) seedAdmin(ctx context.Context) error {
	if _, err := m.re.GetUserByUsername(ctx, "admin"); err == nil {
		return nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash seed password: %w", err)
	}
	return m.re.CreateUser(ctx, &model.DockgeUser{
		Username: "admin",
		Nickname: "管理员",
		Password: string(hashed),
		Role:     model.RoleAdmin,
		Active:   true,
		Source:   model.SourceLocal,
	})
}
