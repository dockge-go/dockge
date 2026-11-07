//go:build linux

package repository

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestContainerMetricsFailClosed(t *testing.T) {
	if _, err := os.Stat("/host/proc/stat"); err == nil {
		t.Skip("执行环境已显式挂载宿主 procfs")
	}
	t.Setenv("container", "dockge-test")
	r := &Repository{}
	q := r.HostSystem(context.Background())
	if q.Source != "" || q.CPUPercent != nil || q.Memory != nil || q.Disk != nil || !strings.Contains(q.Errors["source"], "/host/proc") || !strings.Contains(q.Errors["source"], "/proc/meminfo") {
		t.Fatalf("container fallback must not invent host values: %+v", q)
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
