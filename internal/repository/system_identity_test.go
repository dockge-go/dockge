package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "dockge/api/v1"
)

func TestHostIdentitySources(t *testing.T) {
	for _, raw := range []string{
		`{"Name":"real-host","OperatingSystem":"Linux host","KernelVersion":"6.1","Architecture":"x86_64","NCPU":4,"MemTotal":1024}`,
		`{"host":{"hostname":"real-host","distribution":{"distribution":"Linux","version":"host"},"kernel":"6.1","arch":"x86_64","cpus":4,"memTotal":1024}}`,
	} {
		id, err := parseHostIdentity(raw)
		cpus := 4
		q := &v1.HostSystem{Kernel: "6.1", CPUCount: &cpus, Memory: &v1.HostMemory{Total: 1024}}
		if err != nil || validateHostIdentity(id, q) != nil || id.Name != "real-host" || id.OS != "Linux host" {
			t.Fatalf("%+v, %v", id, err)
		}
		q.Memory.Total = 512
		if validateHostIdentity(id, q) == nil {
			t.Fatal("不能合并不匹配的主机来源")
		}
	}
	if _, err := parseHostIdentity("bad-json"); err == nil {
		t.Fatal("必须保留 JSON 解析错误")
	}
	if validateHostIdentity(hostIdentity{}, &v1.HostSystem{}) == nil {
		t.Fatal("不能猜测缺失主机身份")
	}
}

func TestRemoteIdentityIsNotMixedWithLocalKernel(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://remote-host:2376")
	t.Setenv("CONTAINER_HOST", "")
	r := &Repository{}
	if _, err := r.readHostIdentity(context.Background()); err == nil || !strings.Contains(err.Error(), "remote-host") {
		t.Fatal(err)
	}
}

func TestDockerContextCheckedBeforeIdentity(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("CONTAINER_HOST", "")
	bin := filepath.Join(t.TempDir(), "docker")
	script := "#!/bin/sh\nif [ \"$1\" = context ]; then echo '\"ssh://remote-host\"'; else echo 'unexpected info query' >&2; exit 1; fi\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Repository{runtime: DetectRuntime(bin, "")}
	if _, err := r.readHostIdentity(context.Background()); err == nil || !strings.Contains(err.Error(), "ssh://remote-host") {
		t.Fatal(err)
	}
}

// TestCachedHostIdentity 保持主机身份缓存与每轮内核采样一致，不能用旧身份掩盖来源变化。
func TestCachedHostIdentity(t *testing.T) {
	now := time.Now()
	r := &Repository{systemIdentity: hostIdentity{Name: "real-host", OS: "Linux host", Kernel: "6.1", CPUs: 4, Memory: 1024}, systemIdentityAt: now}
	for _, memory := range []uint64{1024, 512} {
		cpus := 4
		q := &v1.HostSystem{Kernel: "6.1", CPUCount: &cpus, Memory: &v1.HostMemory{Total: memory}, Errors: map[string]string{}}
		r.runtimeHostIdentity(context.Background(), q, now.Add(time.Second))
		if memory == 1024 {
			if q.Hostname != "real-host" || q.OS != "Linux host" || len(q.Errors) != 0 {
				t.Fatalf("缓存身份未正确使用: %+v", q)
			}
		} else if q.Errors["identity"] == "" || q.Hostname != "" || q.OS != "" {
			t.Fatalf("不应混入不同来源的缓存身份: %+v", q)
		}
	}
}

func TestCanceledHostIdentityDoesNotPoisonCache(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///test.sock")
	t.Setenv("CONTAINER_HOST", "")
	r := preflightRuntime(t, `echo '{"Name":"real-host","OperatingSystem":"Linux host","KernelVersion":"6.1","NCPU":4,"MemTotal":1024}'`+"\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	now := time.Now()
	newSample := func() *v1.HostSystem {
		cpus := 4
		return &v1.HostSystem{Kernel: "6.1", CPUCount: &cpus, Memory: &v1.HostMemory{Total: 1024}, Errors: map[string]string{}}
	}
	q := newSample()
	r.runtimeHostIdentity(ctx, q, now)
	if !strings.Contains(q.Errors["identity"], context.Canceled.Error()) {
		t.Fatalf("取消原因丢失: %+v", q)
	}
	if !r.systemIdentityAt.IsZero() || errors.Is(r.systemIdentityError, context.Canceled) {
		t.Fatalf("取消结果被缓存: %v %v", r.systemIdentityAt, r.systemIdentityError)
	}
	q = newSample()
	r.runtimeHostIdentity(context.Background(), q, now.Add(time.Second))
	if q.Hostname != "real-host" || q.Errors["identity"] != "" {
		t.Fatalf("正常查询被取消污染: %+v", q)
	}
}
