// Package repository 是 dockge 应用的数据访问层：
// - 用户账号/设置存 bbolt（go.etcd.io/bbolt）
// - compose 栈存文件系统（stacks 目录）
// - 容器数据经 docker CLI 子进程
package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	r := &Repository{
		db:        do.MustInvoke[*bbolt.DB](i),
		stacksDir: StacksDirFromConf(conf),
		logger:    do.MustInvoke[*log.Logger](i),
		runtime:   DetectRuntime(conf.GetString("container.cli"), conf.GetString("container.compose")),
	}
	// JWT 密钥解析（显式环境变量 > 库中持久值 > 首启随机生成并写库），
	// 结果写回 viper：后续 jwt.Package 惰性解析时拿到的即最终值。
	if err := r.ensureJWTKey(context.Background(), conf); err != nil {
		return nil, err
	}
	return r, nil
}

// ensureJWTKey 保证 JWT 密钥可用且跨重启稳定：
// 显式设置的非弱值直接沿用；否则读库，库中也没有则生成随机值持久化。
// 用户零配置部署时不再落在众人皆知的默认密钥上。
func (r *Repository) ensureJWTKey(ctx context.Context, conf *viper.Viper) error {
	explicit := strings.TrimSpace(conf.GetString("security.jwt.key"))
	if explicit != "" && explicit != "change-me-in-production" {
		return nil
	}
	if saved, err := r.GetSetting(ctx, "jwtKey"); err == nil && saved != "" {
		conf.Set("security.jwt.key", saved)
		return nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate jwt key: %w", err)
	}
	generated := hex.EncodeToString(raw)
	if err := r.SetSetting(ctx, "jwtKey", generated, "security"); err != nil {
		return fmt.Errorf("persist jwt key: %w", err)
	}
	conf.Set("security.jwt.key", generated)
	return nil
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

// StacksDirFromConf 解析 stacks 目录：DOCKGE_STACKS_DIR 环境变量
// （viper 前缀绑定，与上游 dockge 同名，兼容既有编排）→ 缺省 storage/stacks。
func StacksDirFromConf(conf *viper.Viper) string {
	dir := strings.TrimSpace(conf.GetString("stacks_dir"))
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
