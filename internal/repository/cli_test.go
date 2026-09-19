package repository

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// 回归：streamCmd 曾漏掉 Wait()，导致调用方在通道关闭后读到
// ProcessState==nil、ExitCode() 恒为 -1，流式栈操作被误报失败。
func TestStreamCmdSetsProcessState(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want int
	}{
		{"成功退出", []string{"sh", "-c", "echo hi"}, 0},
		{"非零退出", []string{"sh", "-c", "echo err >&2; exit 3"}, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(c.argv[0], c.argv[1:]...)
			ch, err := streamCmd(context.Background(), cmd)
			if err != nil {
				t.Fatalf("streamCmd: %v", err)
			}
			var lines []string
			for line := range ch {
				lines = append(lines, line)
			}
			if cmd.ProcessState == nil {
				t.Fatal("通道关闭后 ProcessState 为 nil（缺 Wait）")
			}
			if code := cmd.ProcessState.ExitCode(); code != c.want {
				t.Fatalf("ExitCode = %d, want %d（输出 %q）", code, c.want, lines)
			}
		})
	}
}

// 取消路径：ctx 取消后通道必须关闭，且 Wait 已收尸（不悬挂 goroutine）。
func TestStreamCmdCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd := exec.Command("sh", "-c", "sleep 5")
	ch, err := streamCmd(ctx, cmd)
	if err != nil {
		t.Fatalf("streamCmd: %v", err)
	}
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ctx 取消后通道未关闭")
	}
}
