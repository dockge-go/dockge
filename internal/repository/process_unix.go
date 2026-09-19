//go:build unix

package repository

import (
	"os/exec"
	"syscall"
)

// setProcessGroup 让子进程独立成组：超时终止时可整组回收，
// 避免孙进程（compose 插件等）持有管道导致读取悬挂。
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup 终止整个进程组。
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
