package repository

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseCPU(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		total, idle uint64
		count       int
		bad         bool
	}{
		{"guest 不重复计数", "cpu 100 2 30 400 10 3 4 5 20 1\ncpu0 1\ncpu1 1", 554, 410, 2, false},
		{"早期格式", "cpu 100 2 30 400\ncpu0 1", 532, 400, 1, false},
		{"少字段", "cpu 1 2\ncpu0 1", 0, 0, 0, true},
		{"无总计", "cpu0 1", 0, 0, 0, true},
		{"无逻辑核", "cpu 1 2 3 4", 0, 0, 0, true},
		{"负计数", "cpu -1 2 3 4\ncpu0 1", 0, 0, 0, true},
		{"溢出", "cpu 18446744073709551615 2 3 4\ncpu0 1", 0, 0, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, n, err := parseCPU(tt.input)
			if (err != nil) != tt.bad {
				t.Fatalf("err=%v", err)
			}
			if !tt.bad && (c.total != tt.total || c.idle != tt.idle || n != tt.count) {
				t.Fatalf("got %+v count=%d", c, n)
			}
		})
	}
}
func TestCPUUsage(t *testing.T) {
	for _, tt := range []struct {
		name      string
		old, next cpuCounters
		want      float64
		bad       bool
	}{
		{"真实差分", cpuCounters{100, 40}, cpuCounters{200, 120}, 20, false},
		{"空闲", cpuCounters{100, 40}, cpuCounters{200, 140}, 0, false},
		{"全忙", cpuCounters{100, 40}, cpuCounters{200, 40}, 100, false},
		{"相同样本", cpuCounters{100, 40}, cpuCounters{100, 40}, 0, true},
		{"重置", cpuCounters{100, 40}, cpuCounters{90, 30}, 0, true},
		{"idle 回退", cpuCounters{100, 40}, cpuCounters{200, 30}, 0, true},
		{"idle 变化溢出区间", cpuCounters{100, 40}, cpuCounters{200, 150}, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pct, err := cpuUsage(tt.old, tt.next)
			if (err != nil) != tt.bad {
				t.Fatalf("err=%v", err)
			}
			if tt.bad {
				if pct != nil {
					t.Fatal("invalid interval cannot show zero")
				}
			} else if pct == nil || math.Abs(*pct-tt.want) > .001 {
				t.Fatalf("pct=%v want=%f", pct, tt.want)
			}
		})
	}
}

const memFixture = "MemTotal: 1000 kB\nMemFree: 100 kB\nMemAvailable: 700 kB\nSwapTotal: 500 kB\nSwapFree: 400 kB"

func TestMemoryAvailable(t *testing.T) {
	mem, swap, err := parseMemory(memFixture)
	if err != nil {
		t.Fatal(err)
	}
	if mem.Total != 1024000 || mem.Used != 307200 || mem.Percent != 30 || swap.Percent != 20 {
		t.Fatalf("memory=%+v swap=%+v", mem, swap)
	}
	_, swap, err = parseMemory(strings.ReplaceAll(strings.ReplaceAll(memFixture, "500 kB", "0 kB"), "400 kB", "0 kB"))
	if err != nil || swap.Percent != 0 || swap.Total != 0 {
		t.Fatalf("disabled swap=%+v err=%v", swap, err)
	}
	for _, input := range []string{"", strings.Replace(memFixture, "MemAvailable: 700 kB", "", 1), strings.Replace(memFixture, "700 kB", "1001 kB", 1), strings.Replace(memFixture, "1000 kB", "0 kB", 1), strings.Replace(memFixture, "700 kB", "NaN kB", 1), strings.Replace(memFixture, "700 kB", "700 MB", 1), strings.Replace(memFixture, "400 kB", "501 kB", 1)} {
		mem, _, err := parseMemory(input)
		if err == nil || mem != nil {
			t.Fatalf("invalid meminfo fabricated a value: %q", input)
		}
	}
}
func TestNumericProcValues(t *testing.T) {
	for _, input := range []string{"NaN 1 2", "Inf 1 2", "-1 1 2", "1 2", ""} {
		if _, err := parseNumbers(input, 3); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	got, err := parseNumbers("0.2 1.3 2.4 1/40 9", 3)
	if err != nil || len(got) != 3 || got[1] != 1.3 {
		t.Fatalf("%v %v", got, err)
	}
}
func TestHostSamplesAreIndependent(t *testing.T) {
	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	write := func(path, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(proc, "stat"), "cpu 100 2 30 400 10 3 4 5\ncpu0 1\ncpu1 1")
	write(filepath.Join(proc, "meminfo"), memFixture)
	write(filepath.Join(proc, "loadavg"), "0.2 1.3 2.4 1/40 9")
	write(filepath.Join(proc, "uptime"), "90061.1 400.5")
	write(filepath.Join(proc, "sys/kernel/osrelease"), "6.12.0")
	write(filepath.Join(proc, "cpuinfo"), "model name : Real Test CPU")
	write(filepath.Join(root, "etc/hostname"), "node-test")
	write(filepath.Join(root, "etc/os-release"), "NAME=Test\nPRETTY_NAME=\"Test Linux\"")
	q, current := sampleHost(hostSource{proc: proc, etc: filepath.Join(root, "etc"), disk: root, name: "host-mount"}, time.Now())
	if q.CPUPercent != nil || current == nil || q.CPUCount == nil || *q.CPUCount != 2 || q.Hostname != "node-test" || q.OS != "Test Linux" || q.Memory == nil || q.Uptime == nil {
		t.Fatalf("incomplete snapshot %+v", q)
	}
	q, _ = sampleHost(hostSource{proc: proc, etc: filepath.Join(root, "etc"), disk: "", name: "host-mount"}, time.Now())
	if q.Memory == nil || q.Disk != nil || !strings.Contains(q.Errors["disk"], "采样路径未配置") {
		t.Fatalf("missing root must not mask CPU/memory or invent disk: %+v", q)
	}
	if err := os.Remove(filepath.Join(proc, "stat")); err != nil {
		t.Fatal(err)
	}
	q, current = sampleHost(hostSource{proc: proc, etc: filepath.Join(root, "etc"), disk: root, name: "host-mount"}, time.Now())
	if current != nil || q.CPUCount != nil || !strings.Contains(q.Errors["cpu"], "stat") || q.Memory == nil {
		t.Fatalf("partial failure %+v", q)
	}
}
