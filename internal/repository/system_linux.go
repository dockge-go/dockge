//go:build linux

package repository

import (
	"fmt"
	"os"
	"strings"

	v1 "dockge/api/v1"
	"golang.org/x/sys/unix"
)

func localHostSource(stacksDir string) (hostSource, error) {
	if info, err := os.Stat("/host/proc/stat"); err == nil && !info.IsDir() {
		return hostSource{proc: "/host/proc", etc: "/host/etc", disk: stacksDir, name: "host-mount"}, nil
	}
	container := os.Getenv("container") != ""
	for _, p := range []string{"/.dockerenv", "/run/.containerenv", "/run/systemd/container"} {
		if _, err := os.Stat(p); err == nil {
			container = true
		}
	}
	if b, err := os.ReadFile(mountinfoPath); err == nil && strings.Contains(string(b), " - overlay ") {
		container = true
	}
	// 标准 Linux procfs 的 CPU/内存计数属于内核全局，不是 cgroup 配额或 docker stats 合计。
	// LXCFS 等会覆盖单个文件，必须逐个检查，不能把虚拟化的容器计数冒充宿主。
	if err := validateNativeProcfs("/proc"); err != nil {
		return hostSource{}, err
	}
	if container {
		return hostSource{proc: "/proc", disk: stacksDir, name: "kernel-proc"}, nil
	}
	return hostSource{proc: "/proc", etc: "/etc", disk: "/", name: "local"}, nil
}

func hostArch() string {
	var name unix.Utsname
	if err := unix.Uname(&name); err != nil {
		return ""
	}
	bytes := name.Machine[:]
	for i, b := range bytes {
		if b == 0 {
			bytes = bytes[:i]
			break
		}
	}
	return string(bytes)
}
func hostDisk(path string) (*v1.HostDisk, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return nil, err
	}
	if stat.Blocks == 0 || stat.Bsize <= 0 || stat.Bfree > stat.Blocks || stat.Bavail > stat.Bfree {
		return nil, fmt.Errorf("statfs 块计数无效：blocks=%d free=%d available=%d blockSize=%d", stat.Blocks, stat.Bfree, stat.Bavail, stat.Bsize)
	}
	block := uint64(stat.Bsize)
	if stat.Blocks > ^uint64(0)/block {
		return nil, fmt.Errorf("statfs 容量超出 uint64 范围：blocks=%d blockSize=%d", stat.Blocks, block)
	}
	used := (stat.Blocks - stat.Bfree) * block
	available := stat.Bavail * block
	if used+available == 0 {
		return nil, fmt.Errorf("statfs 无可计量的已用或可用块 (%s)", path)
	}
	// 与 df 一致，利用率分母不包含仅管理员可用的保留块。
	return &v1.HostDisk{Path: path, Total: stat.Blocks * block, Used: used, Available: available, Percent: 100 * float64(used) / float64(used+available)}, nil
}

// checkTimeNamespace 拒绝将带偏移的容器运行时长标为服务器开机时长。
func checkTimeNamespace() error {
	b, err := os.ReadFile("/proc/self/timens_offsets")
	if os.IsNotExist(err) {
		return nil
	} // 老内核不支持时间命名空间。
	if err != nil {
		return fmt.Errorf("读取时间命名空间偏移: %w", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && (len(f) != 3 || f[1] != "0" || f[2] != "0") {
			return fmt.Errorf("容器时间命名空间存在偏移 %q，无法确认服务器开机时长", line)
		}
	}
	return nil
}

func validateNativeProcfs(proc string) error {
	for _, file := range []string{"stat", "meminfo", "cpuinfo", "loadavg", "uptime", "sys/kernel/osrelease"} {
		path := proc + "/" + file
		var stat unix.Statfs_t
		if err := unix.Statfs(path, &stat); err != nil {
			return fmt.Errorf("验证内核指标来源 %s: %w", path, err)
		}
		if stat.Type != unix.PROC_SUPER_MAGIC {
			return fmt.Errorf("指标文件 %s 并非原生 Linux procfs（文件系统类型 %#x），可能被 LXCFS 等虚拟化；不能作为服务器指标", path, stat.Type)
		}
	}
	return nil
}
