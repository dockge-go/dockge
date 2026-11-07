//go:build !linux

package repository

import (
	v1 "dockge/api/v1"
	"fmt"
	"runtime"
)

func localHostSource(_ string) (hostSource, error) {
	return hostSource{}, fmt.Errorf("服务器采样需要 Linux procfs，当前操作系统为 %s", runtime.GOOS)
}
func hostArch() string { return runtime.GOARCH }
func hostDisk(path string) (*v1.HostDisk, error) {
	return nil, fmt.Errorf("%s 上不支持 Linux 根分区采样 (%s)", runtime.GOOS, path)
}

func checkTimeNamespace() error { return nil }
