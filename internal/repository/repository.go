// Package repository 是 dockge 应用的数据访问层：
// - 用户账号/设置存 bbolt（go.etcd.io/bbolt）
// - compose 栈存文件系统（stacks 目录）
// - 容器数据经 Runtime CLI 子进程
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

	v1 "dockge/api/v1"
)

// bbolt bucket 名称。
const (
	bucketUsers    = "users"
	bucketSettings = "settings"
)

// ErrNotFound 是"目标记录不存在"的哨兵错误。
var ErrNotFound = errors.New("记录不存在")

// ErrStacksNotWritable 表示栈目录写入失败（只读挂载或权限不足）：
// 调用方需包装底层原因与路径，便于定位挂载和权限问题。
var ErrStacksNotWritable = errors.New("栈目录不可写")

// ErrConflict 是"目标已存在"的哨兵错误。

var ErrConflict = errors.New("记录已存在")

// Package 注册仓储层依赖。
var Package = do.Package(
	do.Lazy(NewBBolt),
	do.Lazy(New),
)

// Repository 是仓储层共享的基础设施：bbolt 数据库、stacks 目录与日志。
type Repository struct {
	db        *bbolt.DB
	stacksDir string
	// runtime 是容器运行时命令行（docker/podman/nerdctl 等，可配置/自动探测）
	runtime Runtime
	// preflight* 缓存运行时自检结果：健康检查会周期调用，避免每次起子进程
	preflightMu     sync.Mutex
	preflightAt     time.Time
	preflightStatus RuntimeStatus
	// system* 串行差分采样并短时缓存，多个浏览器不会产生互相污染的 CPU 区间。
	systemMu            sync.Mutex
	systemAt            time.Time
	systemPrevious      *cpuCounters
	systemSnapshot      *v1.HostSystem
	systemIdentityAt    time.Time
	systemIdentity      hostIdentity
	systemIdentityError error
}

// New 构造仓储基础对象，由注入容器调用。
func New(i do.Injector) (*Repository, error) {
	conf := do.MustInvoke[*viper.Viper](i)
	r := &Repository{
		db:        do.MustInvoke[*bbolt.DB](i),
		stacksDir: StacksDirFromConf(conf),
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

// StackComposeCommand 为终端沿用与生命周期操作相同的 compose 文件选择。
func (r *Repository) StackComposeCommand(ctx context.Context, name string, extra ...string) (*exec.Cmd, error) {
	dir, files, err := r.resolveStackExec(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := r.checkStackOwnership(ctx, name); err != nil {
		return nil, err
	}
	cmd, err := r.ComposeCommand(ctx, composeArgs(name, files, extra...)...)
	if cmd != nil {
		cmd.Dir = dir
	}
	return cmd, err
}

// StackPath 返回指定栈的目录路径（已校验名的调用方负责传入合法 name）。
func (r *Repository) StackPath(name string) string {
	return filepath.Join(r.stacksDir, name)
}

// StacksDir 返回栈根目录。
func (r *Repository) StacksDir() string {
	return r.stacksDir
}

// NewBBolt 打开 bbolt 数据库文件并返回连接。
func NewBBolt(i do.Injector) (*bbolt.DB, error) {
	conf := do.MustInvoke[*viper.Viper](i)
	dsn := conf.GetString("data.db.user.dsn")
	if dsn == "" {
		dsn = "/app/data/dockge.db"
	}
	// 确保父目录存在
	if dir := filepath.Dir(dsn); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create bbolt db dir: %w", err)
		}
	}
	// 单机约束：同一数据库同时只允许一个进程持有。加锁超时快速失败，
	// 第二个实例超过等待期限即返回锁冲突，避免无限挂起。
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

// InitBuckets 是 initBuckets 的导出别名：service 层测试需要在自建的临时库上准备 bucket
// （Repository 构造走 do.Injector，测试无法注入，故以最小构造器暴露）。
func InitBuckets(db *bbolt.DB) error {
	return initBuckets(db)
}

// NewRepositoryForTest 以仅含 db 的最小构造创建 Repository（测试专用：其余字段为 nil，
// 只可调用用户/设置等纯存储方法，禁止触碰 runtime/stacksDir 相关方法）。
func NewRepositoryForTest(db *bbolt.DB) *Repository {
	return &Repository{db: db}
}

// StacksDirFromConf 解析 stacks 目录：DOCKGE_STACKS_DIR 环境变量
// （viper 前缀绑定，与上游 dockge 同名，兼容既有编排）→ 缺省 /opt/stacks。
func StacksDirFromConf(conf *viper.Viper) string {
	dir := strings.TrimSpace(conf.GetString("stacks_dir"))
	if dir == "" {
		return "/opt/stacks"
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
