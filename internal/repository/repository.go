// Package repository 是 dockge 应用的数据访问层：
// - 用户账号/设置存 bbolt（go.etcd.io/bbolt）
// - compose 栈存文件系统（stacks 目录）
// - 容器数据经 docker CLI 子进程
package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"

	"github.com/samber/do/v2"
	"github.com/spf13/viper"

	"dockge/pkg/log"
)

// bbolt bucket 名称。
const (
	bucketUsers    = "users"
	bucketSettings = "settings"
)

// ErrNotFound 是"目标记录不存在"的哨兵错误。
var ErrNotFound = errors.New("record not found")

// ErrStacksNotWritable 表示栈目录写入失败（只读挂载或权限不足）：
// 该错误必须带上真实原因与路径，否则用户只看到「服务器错误~」，无从知道是挂载问题。
var ErrStacksNotWritable = errors.New("栈目录不可写")

// ErrConflict 是"目标已存在"的哨兵错误。

var ErrConflict = errors.New("record already exists")

// Package registers all repository-layer providers into the injector.
var Package = do.Package(
	do.Lazy(NewBBolt),
	do.Lazy(New),
)

// Repository 是仓储层共享的基础设施：bbolt 数据库、stacks 目录与日志。
type Repository struct {
	db        *bbolt.DB
	stacksDir string
	logger    *log.Logger
	// runtime 是容器运行时命令行（docker/podman/nerdctl 等，可配置/自动探测）
	runtime Runtime
	// preflight* 缓存运行时自检结果：健康检查会周期调用，避免每次起子进程
	preflightOnce   sync.Once
	preflightStatus RuntimeStatus
}

// New 构造仓储基础对象，由注入容器调用。
func New(i do.Injector) (*Repository, error) {
	conf := do.MustInvoke[*viper.Viper](i)
	return &Repository{
		db:        do.MustInvoke[*bbolt.DB](i),
		stacksDir: StacksDirFromConf(conf),
		logger:    do.MustInvoke[*log.Logger](i),
		runtime:   DetectRuntime(conf.GetString("container.cli"), conf.GetString("container.compose")),
	}, nil
}

// ContainerCommand 构造容器命令（供 handler 层使用，如容器终端）。
func (r *Repository) ContainerCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	return r.runtime.Command(ctx, args...)
}

// ComposeCommand 构造 compose 命令（供 handler 层使用，如合并日志终端）。
func (r *Repository) ComposeCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	return r.runtime.ComposeCommand(ctx, args...)
}

// StackPath 返回指定栈的目录路径（已校验名的调用方负责传入合法 name）。
func (r *Repository) StackPath(name string) string {
	return filepath.Join(r.stacksDir, name)
}

// StacksDir 返回栈根目录（宿主 shell 终端的工作目录）。
func (r *Repository) StacksDir() string {
	return r.stacksDir
}

// NewBBolt 打开 bbolt 数据库文件并返回连接。
func NewBBolt(i do.Injector) (*bbolt.DB, error) {
	conf := do.MustInvoke[*viper.Viper](i)
	dsn := conf.GetString("data.db.user.dsn")
	if dsn == "" {
		dsn = "storage/dockge.db"
	}
	// 确保父目录存在
	if dir := filepath.Dir(dsn); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create bbolt db dir: %w", err)
		}
	}
	// 单机约束：同一数据库同时只允许一个进程持有。加锁超时快速失败，
	// 避免第二个进程（如 reset-password）静默挂起等待文件锁。
	db, err := bbolt.Open(dsn, 0o600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bbolt database %q: %w", dsn, err)
	}
	// 初始化 bucket（幂等）
	if err := initBuckets(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init bbolt buckets: %w", err)
	}
	return db, nil
}

// initBuckets 确保所有业务 bucket 存在。
func initBuckets(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		for _, name := range []string{bucketUsers, bucketSettings} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bucket %q: %w", name, err)
			}
		}
		return nil
	})
}

// StacksDirFromConf 解析 stacks 目录，优先级：
// DOCKGE_STACKS_DIR 环境变量（上游同名，兼容从上游迁移的编排）→
// 配置/APP_DOCKGE_STACKS_DIR → 缺省 storage/stacks。
func StacksDirFromConf(conf *viper.Viper) string {
	if env := strings.TrimSpace(os.Getenv("DOCKGE_STACKS_DIR")); env != "" {
		return env
	}
	dir := conf.GetString("dockge.stacks_dir")
	if dir == "" {
		return "storage/stacks"
	}
	return dir
}

// TxBucket 辅助：在事务中获取指定 bucket 并执行 fn。
func TxBucket(tx *bbolt.Tx, bucket string) (*bbolt.Bucket, error) {
	b := tx.Bucket([]byte(bucket))
	if b == nil {
		return nil, fmt.Errorf("bucket %q not found", bucket)
	}
	return b, nil
}
