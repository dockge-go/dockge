package pty

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// Open 打开伪终端（master），把子进程的 stdio 接到 slave 端并启动进程。
// 返回的 master 用于读写子进程的终端 I/O；调用方负责在退出时
// 关闭 master 并 Kill 进程。
func Open(cmd *exec.Cmd) (*os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	if err := unlockSlave(master); err != nil {
		master.Close()
		return nil, err
	}
	slavePath, err := slavePath(master)
	if err != nil {
		master.Close()
		return nil, err
	}
	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		master.Close()
		return nil, err
	}
	defer slave.Close()

	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	// 新会话 + slave 作为控制终端，Ctrl+C/SIGWINCH 等信号才能正确送达
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		master.Close()
		return nil, err
	}
	return master, nil
}

// slavePath 通过 TIOCGPTN 取出 slave 端设备路径（/dev/pts/N）。
func slavePath(master *os.File) (string, error) {
	var n uint32
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		master.Fd(),
		syscall.TIOCGPTN,
		uintptr(unsafe.Pointer(&n)),
	)
	if errno != 0 {
		return "", errno
	}
	return fmt.Sprintf("/dev/pts/%d", n), nil
}

// unlockSlave 清除 TIOCSPTLCK 锁，否则 slave 端打开会报 EIO。
func unlockSlave(master *os.File) error {
	unlock := int32(0)
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		master.Fd(),
		syscall.TIOCSPTLCK,
		uintptr(unsafe.Pointer(&unlock)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// Winsize 是终端窗口大小。
type Winsize struct {
	Rows uint16
	Cols uint16
	X    uint16
	Y    uint16
}

// SetWinsize 设置 PTY 窗口大小。
func SetWinsize(f *os.File, w *Winsize) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(w)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}
