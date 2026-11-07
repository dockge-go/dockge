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
	// 容器缺少宿主挂载时拒绝回退：容器的 proc、UTS 与根分区可能经过虚拟化。
	container := os.Getenv("KUBERNETES_SERVICE_HOST") != "" || os.Getenv("container") != ""
	for _, p := range []string{"/.dockerenv", "/run/.containerenv", "/run/systemd/container"} {
		if _, err := os.Stat(p); err == nil {
			container = true
		}
	}
	if b, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) > 5 && f[4] == "/" && strings.Contains(line, " - overlay ") {
				container = true
			}
		}
	}
	if container {
		return hostSource{}, fmt.Errorf("容器未配置可信宿主指标来源：请按部署清单只读挂载 /proc/stat、/proc/meminfo 等固定统计文件到 /host/proc（不需要挂载整个 /proc 或宿主根目录）；不会把容器指标当成服务器指标")
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
