package repository

// Podman client 连接与选项助手：所有 docker/podman 关联操作
// 优先走 podman REST client（本文件），不可用时统一降级 docker CLI。

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"go.podman.io/podman/v6/pkg/bindings"
)

type PodmanClient struct {
	ctx   context.Context
	avail bool
}

// DefaultPodmanSocket 返回默认 podman socket 路径（rootful 优先，其次 rootless）。
func DefaultPodmanSocket() string {
	uid := strconv.FormatUint(uint64(os.Getuid()), 10)
	candidates := []string{
		"/run/podman/podman.sock",
		filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "podman", "podman.sock"),
		filepath.Join("/run/user", uid, "podman", "podman.sock"),
	}
	for _, s := range candidates {
		if _, err := os.Stat(s); err == nil {
			return "unix://" + s
		}
	}
	return ""
}

// NewPodmanClient 尝试连接到 podman socket；失败时返回 avail=false。
func NewPodmanClient() (*PodmanClient, error) {
	socket := DefaultPodmanSocket()
	if socket == "" {
		return &PodmanClient{avail: false}, nil
	}
	ctx, err := bindings.NewConnection(context.Background(), socket)
	if err != nil {
		return &PodmanClient{avail: false}, nil
	}
	return &PodmanClient{ctx: ctx, avail: true}, nil
}

// IsAvailable 是否可用。
func (c *PodmanClient) IsAvailable() bool { return c.avail }

// boolPtr/uintPtr/strPtr 是 podman bindings 选项字段的指针包装。
func boolPtr(b bool) *bool    { return &b }
func uintPtr(u uint) *uint    { return &u }
func strPtr(s string) *string { return &s }
