//go:build !linux

// Package pty 在非 Linux 平台的 stub：宿主终端能力依赖 Linux 专有 ioctl，
// 构建通过但 Open 返回错误，调用方（terminal handler）向 WebSocket 写出
// 错误消息后关闭连接——面板其余功能不受影响（D10 平台声明的另一半）。
package pty

import (
	"errors"
	"os"
	"os/exec"
)

// ErrUnsupported 平台不支持 PTY（仅 Linux）。
var ErrUnsupported = errors.New("pty: 仅支持 Linux（本构建运行在非 Linux 平台）")

// Open 在非 Linux 平台始终失败。
func Open(cmd *exec.Cmd) (*os.File, error) { return nil, ErrUnsupported }

// SetWinsize 在非 Linux 平台为无操作。
func SetWinsize(f *os.File, ws *Winsize) error { return nil }

// Winsize 与 Linux 版保持同构，供控制消息反序列化。
type Winsize struct {
	Rows uint16
	Cols uint16
}
