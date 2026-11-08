//go:build unix && !linux

package repository

import "os/exec"

// waitForProcessExit 安全降级为普通 Wait，不假设其他 Unix 的 waitid ABI。
// 这些平台不能保证正常退出后清理后台后代；收尸后不再按数字 PGID 杀组。
func waitForProcessExit(cmd *exec.Cmd) (waited bool, err error) {
	return true, cmd.Wait()
}

// killCommandProcessGroup 降级为 os.Process 的安全单进程终止。
// 普通 Wait 会并发收尸，不能沿用负 PID 杀组而引入 PGID 重用竞态。
func killCommandProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
