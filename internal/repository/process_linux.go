//go:build linux

package repository

import (
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

// waitForProcessExit 等组长退出但不收尸；PID 保留期间可安全按 PGID 杀组。
// 即使后代关闭 stdout/stderr，组长的僵尸状态也会保留到最终清理完成。
func waitForProcessExit(cmd *exec.Cmd) (waited bool, err error) {
	var info unix.Siginfo
	for {
		err := unix.Waitid(unix.P_PID, cmd.Process.Pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			// 不支持/拒绝 waitid 时返回具体错误，调用者在 Wait 前终止整组。
			return false, fmt.Errorf("等待命令进程退出（保留 PID 以安全清理进程组）: %w", os.NewSyscallError("waitid", err))
		}
		return false, nil
	}
}

func killCommandProcessGroup(cmd *exec.Cmd) {
	killProcessGroup(cmd)
}
