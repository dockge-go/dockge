// Package repository 是 dockge 应用的数据访问层：
// - 用户账号/设置存 bbolt（go.etcd.io/bbolt）
// - compose 栈存文件系统（stacks 目录）
// - 容器数据来自 podman / docker CLI
package repository

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	podman    *PodmanClient
}

// New 构造仓储基础对象，由注入容器调用。
func New(i do.Injector) (*Repository, error) {
	repo := &Repository{
		db:        do.MustInvoke[*bbolt.DB](i),
		stacksDir: StacksDirFromConf(do.MustInvoke[*viper.Viper](i)),
		logger:    do.MustInvoke[*log.Logger](i),
	}
	if pc, err := NewPodmanClient(); err == nil {
		repo.podman = pc
	}
	return repo, nil
}

// StackPath 返回指定栈的目录路径（已校验名的调用方负责传入合法 name）。
func (r *Repository) StackPath(name string) string {
	return filepath.Join(r.stacksDir, name)
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
	db, err := bbolt.Open(dsn, 0o600, &bbolt.Options{})
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

// StacksDirFromConf 从配置读取 stacks 目录，缺省 storage/stacks。
func StacksDirFromConf(conf *viper.Viper) string {
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
