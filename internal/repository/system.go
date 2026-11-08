package repository

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	v1 "dockge/api/v1"
)

type cpuCounters struct{ total, idle uint64 }
type hostSource struct{ proc, etc, disk, name string }

// HostSystem 用两个真实计数器样本计算 CPU；首样本为空而不是伪造 0%。
func (r *Repository) HostSystem(ctx context.Context) *v1.HostSystem {
	r.systemMu.Lock()
	defer r.systemMu.Unlock()
	now := time.Now()
	if r.systemSnapshot != nil && now.Sub(r.systemAt) < time.Second {
		return r.systemSnapshot
	}
	q := &v1.HostSystem{SampledAt: now.UTC().Format(time.RFC3339Nano), Errors: map[string]string{}}
	if err := ctx.Err(); err != nil {
		q.Errors["request"] = err.Error()
		return q
	}
	src, err := localHostSource(r.stacksDir)
	if err != nil {
		q.Errors["source"] = err.Error()
		r.systemPrevious = nil
		if r.stacksDir != "" {
			q.Disk, err = hostDisk(r.stacksDir)
			if err != nil {
				q.Errors["disk"] = err.Error()
			}
		}
	} else {
		var current *cpuCounters
		q, current = sampleHost(src, now)
		if src.name == "kernel-proc" {
			r.runtimeHostIdentity(ctx, q, now)
		}
		if err := ctx.Err(); err != nil {
			q.Errors["request"] = err.Error()
			return q
		}
		if current != nil {
			if r.systemPrevious != nil {
				q.CPUPercent, err = cpuUsage(*r.systemPrevious, *current)
				if err != nil {
					q.Errors["cpu"] = err.Error()
				}
			}
			r.systemPrevious = current
		} else {
			r.systemPrevious = nil
		}
	}
	r.systemAt, r.systemSnapshot = now, q
	return q
}

func sampleHost(src hostSource, now time.Time) (*v1.HostSystem, *cpuCounters) {
	q := &v1.HostSystem{Source: src.name, SampledAt: now.UTC().Format(time.RFC3339Nano), Errors: map[string]string{}}
	read := func(key, path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			q.Errors[key] = fmt.Sprintf("读取服务器 %s (%s)：%v", key, path, err)
			return ""
		}
		value := strings.TrimSpace(string(b))
		if value == "" {
			q.Errors[key] = fmt.Sprintf("读取服务器 %s (%s)：文件为空", key, path)
		}
		return value
	}
	var current *cpuCounters
	if s := read("cpu", filepath.Join(src.proc, "stat")); s != "" {
		c, count, err := parseCPU(s)
		if err != nil {
			q.Errors["cpu"] = err.Error()
		} else {
			current = &c
			q.CPUCount = &count
		}
	}
	if s := read("memory", filepath.Join(src.proc, "meminfo")); s != "" {
		mem, swap, err := parseMemory(s)
		if err != nil {
			q.Errors["memory"] = err.Error()
		} else {
			q.Memory = mem
			q.Swap = swap
		}
	}
	if s := read("load", filepath.Join(src.proc, "loadavg")); s != "" {
		values, err := parseNumbers(s, 3)
		if err != nil {
			q.Errors["load"] = "解析服务器 loadavg：" + err.Error()
		} else {
			q.Load = values
		}
	}
	if s := read("uptime", filepath.Join(src.proc, "uptime")); s != "" {
		values, err := parseNumbers(s, 1)
		if err != nil {
			q.Errors["uptime"] = "解析服务器 uptime：" + err.Error()
		} else {
			q.Uptime = &values[0]
		}
	}
	q.Kernel = read("kernel", filepath.Join(src.proc, "sys/kernel/osrelease"))
	q.Arch = hostArch()
	if s := read("cpuModel", filepath.Join(src.proc, "cpuinfo")); s != "" {
		for _, line := range strings.Split(s, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if ok && (strings.TrimSpace(key) == "model name" || strings.TrimSpace(key) == "Hardware") {
				q.CPUModel = strings.TrimSpace(value)
				break
			}
		}
	}
	// 容器的 /etc 属于镜像和 UTS 命名空间，不能用它冒充宿主身份。
	if src.etc != "" {
		q.Hostname = read("hostname", filepath.Join(src.etc, "hostname"))
		if src.name == "local" {
			if name, err := os.Hostname(); err == nil {
				q.Hostname = name
				delete(q.Errors, "hostname")
			}
		}
		if s := read("os", filepath.Join(src.etc, "os-release")); s != "" {
			for _, line := range strings.Split(s, "\n") {
				if value, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
					q.OS = strings.Trim(value, "\"'")
					break
				}
			}
		}
	}
	if src.name == "kernel-proc" {
		if err := checkTimeNamespace(); err != nil {
			q.Uptime = nil
			q.Errors["uptime"] = err.Error()
		}
	}
	if src.disk == "" {
		q.Errors["disk"] = "栈目录文件系统采样路径未配置，不能以容器根分区代替"
	} else {
		var err error
		q.Disk, err = hostDisk(src.disk)
		if err != nil {
			q.Errors["disk"] = fmt.Sprintf("读取文件系统空间 (%s)：%v", src.disk, err)
		}
	}
	return q, current
}

func parseCPU(s string) (cpuCounters, int, error) {
	var counters cpuCounters
	count, found := 0, false
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if strings.HasPrefix(f[0], "cpu") && f[0] != "cpu" {
			if _, err := strconv.Atoi(strings.TrimPrefix(f[0], "cpu")); err == nil {
				count++
			}
			continue
		}
		if f[0] != "cpu" {
			continue
		}
		if len(f) < 5 {
			return counters, 0, fmt.Errorf("解析 /proc/stat：CPU 总计缺少 user/nice/system/idle，实际 %q", line)
		}
		found = true
		// 前八项包含 steal；guest/guest_nice 已包含在 user/nice 中，不可重复相加。
		for i := 1; i < len(f) && i <= 8; i++ {
			n, err := strconv.ParseUint(f[i], 10, 64)
			if err != nil || math.MaxUint64-counters.total < n {
				return cpuCounters{}, 0, fmt.Errorf("解析 /proc/stat：CPU 计数器第 %d 项 %q 非法或溢出", i, f[i])
			}
			counters.total += n
			if i == 4 || i == 5 {
				counters.idle += n
			}
		}
	}
	if !found || count == 0 {
		return counters, 0, fmt.Errorf("解析 /proc/stat：缺少 CPU 总计或逻辑处理器记录")
	}
	return counters, count, nil
}

func cpuUsage(old, next cpuCounters) (*float64, error) {
	if next.total <= old.total || next.idle < old.idle || next.idle-old.idle > next.total-old.total {
		return nil, fmt.Errorf("CPU 计数器区间无效或已重置：total %d→%d，idle %d→%d；等待下一个有效样本", old.total, next.total, old.idle, next.idle)
	}
	value := 100 * float64((next.total-old.total)-(next.idle-old.idle)) / float64(next.total-old.total)
	return &value, nil
}

func parseMemory(s string) (*v1.HostMemory, *v1.HostMemory, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		key := strings.TrimSuffix(f[0], ":")
		if key != "MemTotal" && key != "MemAvailable" && key != "SwapTotal" && key != "SwapFree" {
			continue
		}
		if len(f) != 3 || f[2] != "kB" {
			return nil, nil, fmt.Errorf("解析 /proc/meminfo：%s 需要以 kB 为单位，实际 %q", key, line)
		}
		n, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil || n > math.MaxUint64/1024 {
			return nil, nil, fmt.Errorf("解析 /proc/meminfo：%s 值 %q 非法或溢出", key, f[1])
		}
		values[key] = n * 1024
	}
	for _, key := range []string{"MemTotal", "MemAvailable", "SwapTotal", "SwapFree"} {
		if _, ok := values[key]; !ok {
			return nil, nil, fmt.Errorf("解析 /proc/meminfo：缺少 %s，不能推测用量", key)
		}
	}
	if values["MemTotal"] == 0 || values["MemAvailable"] > values["MemTotal"] || values["SwapFree"] > values["SwapTotal"] {
		return nil, nil, fmt.Errorf("解析 /proc/meminfo：总量/可用量区间非法 (MemTotal=%d MemAvailable=%d SwapTotal=%d SwapFree=%d)", values["MemTotal"], values["MemAvailable"], values["SwapTotal"], values["SwapFree"])
	}
	makeMem := func(total, available uint64) *v1.HostMemory {
		m := &v1.HostMemory{Total: total, Available: available, Used: total - available}
		if total > 0 {
			m.Percent = 100 * float64(m.Used) / float64(total)
		}
		return m
	}
	return makeMem(values["MemTotal"], values["MemAvailable"]), makeMem(values["SwapTotal"], values["SwapFree"]), nil
}

func parseNumbers(s string, count int) ([]float64, error) {
	f := strings.Fields(s)
	if len(f) < count {
		return nil, fmt.Errorf("需要 %d 个数值，实际 %q", count, s)
	}
	out := make([]float64, count)
	for i := range out {
		n, err := strconv.ParseFloat(f[i], 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			return nil, fmt.Errorf("数值 %q 不是有限非负数", f[i])
		}
		out[i] = n
	}
	return out, nil
}
