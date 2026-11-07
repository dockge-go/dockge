package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "dockge/api/v1"
)

type hostIdentity struct {
	Name, OS, Kernel, Arch string
	CPUs                   int
	Memory                 uint64
}

// runtimeHostIdentity 只从本机运行时读取宿主身份；CPU/内存占用仍来自真实内核计数。
func (r *Repository) runtimeHostIdentity(ctx context.Context, q *v1.HostSystem, now time.Time) {
	if r.systemIdentityAt.IsZero() || now.Sub(r.systemIdentityAt) >= 30*time.Second {
		r.systemIdentity, r.systemIdentityError = r.readHostIdentity(ctx)
		r.systemIdentityAt = now
	}
	if err := r.systemIdentityError; err != nil {
		q.Errors["identity"] = err.Error()
		return
	}
	id := r.systemIdentity
	if err := validateHostIdentity(id, q); err != nil {
		q.Errors["identity"] = err.Error()
		return
	}
	q.Hostname, q.OS = id.Name, id.OS
}

func validateHostIdentity(id hostIdentity, q *v1.HostSystem) error {
	if id.Name == "" || id.OS == "" || id.Kernel == "" || id.CPUs <= 0 || id.Memory == 0 {
		return fmt.Errorf("运行时 info 缺少宿主名称、操作系统、内核、CPU 核数或内存总量；不会以容器身份补齐")
	}
	if q.CPUCount == nil || q.Memory == nil || id.Kernel != q.Kernel || id.CPUs != *q.CPUCount || id.Memory != q.Memory.Total {
		return fmt.Errorf("运行时宿主信息与面板内核采样不一致（运行时 kernel=%q cpus=%d memory=%d；procfs kernel=%q）；不合并远程或虚拟化来源", id.Kernel, id.CPUs, id.Memory, q.Kernel)
	}
	return nil
}

func (r *Repository) readHostIdentity(ctx context.Context) (hostIdentity, error) {
	// Docker context 可以隐式转向远程；unix socket 才符合单机指标的范围。
	endpoint := os.Getenv("DOCKER_HOST")
	if other := os.Getenv("CONTAINER_HOST"); other != "" {
		endpoint = other
	}
	if endpoint == "" && filepath.Base(r.runtime.resolved().bin) == "docker" {
		out, err := r.runDocker(ctx, time.Second, "context", "inspect", "--format", "{{json .Endpoints.docker.Host}}")
		if err != nil {
			return hostIdentity{}, fmt.Errorf("确认本机运行时连接: %w", err)
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &endpoint); err != nil {
			return hostIdentity{}, fmt.Errorf("解析运行时连接地址: %w", err)
		}
	}
	if endpoint != "" && !strings.HasPrefix(endpoint, "unix://") {
		return hostIdentity{}, fmt.Errorf("运行时连接 %q 不是本机 unix socket，服务器采样仅描述面板所在 Linux 内核", endpoint)
	}
	out, err := r.runDocker(ctx, 2*time.Second, "info", "--format", "{{json .}}")
	if err != nil {
		return hostIdentity{}, fmt.Errorf("读取本机运行时宿主信息: %w", err)
	}
	return parseHostIdentity(out)
}

func parseHostIdentity(out string) (hostIdentity, error) {
	var obj struct {
		Name, OperatingSystem, KernelVersion, Architecture string
		NCPU                                               int
		MemTotal                                           uint64
		Host                                               struct {
			Hostname, Kernel, Arch string
			CPUs                   int
			MemTotal               uint64
			Distribution           struct{ Distribution, Version string }
		}
	}
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		return hostIdentity{}, fmt.Errorf("解析运行时宿主信息: %w", err)
	}
	id := hostIdentity{obj.Name, obj.OperatingSystem, obj.KernelVersion, obj.Architecture, obj.NCPU, obj.MemTotal}
	if id.Name == "" && obj.Host.Hostname != "" {
		h := obj.Host
		id = hostIdentity{h.Hostname, strings.TrimSpace(h.Distribution.Distribution + " " + h.Distribution.Version), h.Kernel, h.Arch, h.CPUs, h.MemTotal}
	}
	return id, nil
}
