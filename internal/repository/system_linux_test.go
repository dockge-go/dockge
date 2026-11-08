//go:build linux

package repository

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestContainerUsesRealKernelWithoutExtraMounts(t *testing.T) {
	if _, err := os.Stat("/host/proc/stat"); err == nil {
		t.Skip("执行环境已显式挂载宿主 procfs")
	}
	t.Setenv("container", "dockge-test")
	r := &Repository{stacksDir: t.TempDir()}
	// 仅控制测试时钟，procfs/statfs 仍真实读取；CI 调度与读取耗时不能决定缓存断言。
	synctest.Test(t, func(t *testing.T) {
		q := r.HostSystem(context.Background())
		if q.Source != "kernel-proc" || q.CPUPercent != nil || q.Memory == nil || q.Disk == nil || q.CPUCount == nil {
			t.Fatalf("标准 procfs 必须直接采样，首样本 CPU 不能伪造: %+v", q)
		}
		b, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			t.Fatal(err)
		}
		mem, _, err := parseMemory(string(b))
		if err != nil || q.Memory.Total != mem.Total {
			t.Fatal("内存总量必须来自真实 procfs")
		}
		if r.HostSystem(context.Background()) != q {
			t.Fatal("rapid requests must share the sampling interval")
		}
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				got := r.HostSystem(context.Background())
				if got != q {
					t.Error("concurrent reader did not use cache")
				}
			})
		}
		wg.Wait()
		time.Sleep(time.Second - time.Nanosecond)
		if got := r.HostSystem(context.Background()); got != q {
			t.Fatal("采样间隔到期前必须复用缓存")
		}
		time.Sleep(time.Nanosecond)
		if got := r.HostSystem(context.Background()); got == q || got.SampledAt == q.SampledAt {
			t.Fatal("采样间隔到期后必须重新读取真实内核数据")
		}
	})
}

func TestHostDiskMatchesStatfs(t *testing.T) {
	disk, err := hostDisk("/")
	if err != nil {
		t.Fatal(err)
	}
	if disk.Path != "/" || disk.Total == 0 || disk.Used+disk.Available > disk.Total || disk.Percent < 0 || disk.Percent > 100 {
		t.Fatalf("invalid real filesystem snapshot %+v", disk)
	}
	if value, err := hostDisk("/not-present-dockge-system-test"); err == nil || value != nil {
		t.Fatal("missing filesystem must be unknown")
	}
}

func TestVirtualizedProcfsIsRejected(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "stat"), []byte("cpu 1 1 1 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateNativeProcfs(root); err == nil || !strings.Contains(err.Error(), "stat") || !strings.Contains(err.Error(), "procfs") {
		t.Fatalf("虚拟/普通文件不可作为内核数据: %v", err)
	}
}
