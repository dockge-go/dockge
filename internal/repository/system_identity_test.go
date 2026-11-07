package repository

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
