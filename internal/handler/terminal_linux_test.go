//go:build linux

package handler

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sys/unix"
)

// 描述符关闭不能代表后代退出；日志与 PTY 共用的等待函数必须先清组再收尸。
func TestTerminalNormalExitCleansBackgroundGroup(t *testing.T) {
	for _, usePTY := range []bool{false, true} {
		for _, closedOutput := range []bool{false, true} {
			t.Run(fmt.Sprintf("pty-%t/closed-output-%t", usePTY, closedOutput), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				dir := t.TempDir()
				pidPath, release := filepath.Join(dir, "child.pid"), filepath.Join(dir, "release")
				redirect := ""
				if closedOutput {
					redirect = ">/dev/null 2>&1"
				}
				body := fmt.Sprintf("sleep 30 %s & echo $! > %q; while [ ! -f %q ]; do sleep 0.01; done; printf 'final-output\\n'; exit 0", redirect, pidPath, release)
				cmd := exec.CommandContext(ctx, "sh", "-c", body)
				writer := &terminalEventWriter{ResponseRecorder: httptest.NewRecorder(), frames: make(chan string, 10)}
				c, _ := gin.CreateTestContext(writer)
				c.Request = httptest.NewRequest("GET", "/stream", nil).WithContext(ctx)
				ended := make(chan struct{})
				go func() { defer close(ended); (&TerminalHandler{}).serveSession(c, cmd, usePTY, cancel) }()
				defer func() {
					cancel()
					select {
					case <-ended:
					case <-time.After(3 * time.Second):
						t.Error("测试终端未完成清理")
					}
				}()
				awaitTerminalEvent(t, writer.frames, "ready")
				var pid int
				deadline := time.Now().Add(time.Second)
				for time.Now().Before(deadline) {
					data, _ := os.ReadFile(pidPath)
					pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
					if pid > 0 {
						break
					}
					time.Sleep(time.Millisecond)
				}
				if pid <= 0 {
					t.Fatal("后台后代未启动")
				}
				fd, err := unix.PidfdOpen(pid, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer unix.Close(fd)
				if err := os.WriteFile(release, nil, 0600); err != nil {
					t.Fatal(err)
				}
				select {
				case <-ended:
				case <-time.After(3 * time.Second):
					t.Fatal("组长正常退出后终端未结束")
				}
				poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
				deadline = time.Now().Add(time.Second)
				for {
					n, err := unix.Poll(poll, max(0, int(time.Until(deadline).Milliseconds())))
					if err == unix.EINTR && time.Now().Before(deadline) {
						continue
					}
					if err != nil || n != 1 {
						t.Fatalf("正常退出遗留后台进程: pid=%d poll=%d err=%v", pid, n, err)
					}
					break
				}
				if cmd.ProcessState == nil || !cmd.ProcessState.Success() || strings.Contains(writer.Body.String(), "event: error") {
					t.Fatalf("安全退出丢失真实结束状态: state=%v output=%s", cmd.ProcessState, writer.Body.String())
				}
				if err := cmd.Cancel(); err != nil {
					t.Fatalf("完成后的重复取消不安全: %v", err)
				}
			})
		}
	}
}
