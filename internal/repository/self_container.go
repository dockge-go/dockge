package repository

import (
	"os"
	"regexp"
	"strings"
)

var fullContainerID = regexp.MustCompile(`(?:^|[/:-])([a-f0-9]{64})(?:[/.\s]|$)`)

// containerIdentity 优先使用内核记录，不依赖可自定义的容器 hostname。
func containerIdentity(hostname, cgroup, mounts string) string {
	if m := fullContainerID.FindStringSubmatch(cgroup); m != nil {
		return m[1]
	}
	for _, line := range strings.Split(mounts, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		switch unescapeMountPoint(fields[4]) {
		case "/etc/hostname", "/etc/hosts", "/etc/resolv.conf":
			if m := fullContainerID.FindStringSubmatch(unescapeMountPoint(fields[3])); m != nil {
				return m[1]
			}
		}
	}
	// 常规容器 hostname 为短 ID；裸机上只会匹配真实容器 ID 前缀。
	if len(hostname) >= 12 && len(hostname) <= 64 {
		for _, c := range hostname {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return ""
			}
		}
		return hostname
	}
	return ""
}
func selfContainerIdentity() string {
	hostname, _ := os.Hostname()
	cgroup, _ := os.ReadFile("/proc/self/cgroup")
	mounts, _ := os.ReadFile(mountinfoPath)
	return containerIdentity(hostname, string(cgroup), string(mounts))
}
