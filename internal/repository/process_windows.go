//go:build !unix

package repository

import "os/exec"

// setProcessGroup Windows 无进程组概念：不做处理（子进程由 Kill 直接终止）。
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup 仅终止直接子进程。
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
