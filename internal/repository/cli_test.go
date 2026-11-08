package repository

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
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
			for line := range ch.lines {
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
		for range ch.lines {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ctx 取消后通道未关闭")
	}
}

func TestStreamCmdOversizedOutputReportsFailure(t *testing.T) {
	cmd := exec.Command("sh", "-c", "head -c 1100000 /dev/zero | tr '\\000' x; printf '\\n'")
	stream, err := streamCmd(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	for range stream.lines {
	}
	if stream.err == nil {
		t.Fatal("超长日志被静默截断并误报成功")
	}
}

func TestRunCmdFailurePreservesBothOutputs(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo created-container-id; echo start-failed >&2; exit 1")
	out, err := runCmd(context.Background(), cmd, "", time.Second, []string{"run"})
	if err == nil || !strings.Contains(out, "created-container-id") || !strings.Contains(err.Error(), "start-failed") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestRunCmdPreservesCancellationAndExitCauseWithOutput(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		timeout time.Duration
		want    error
	}{
		{"超时仍保留输出", "echo before-timeout; sleep 5", 40 * time.Millisecond, context.DeadlineExceeded},
		{"非零退出仍保留退出码", "echo before-failure; exit 7", time.Second, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCmd(context.Background(), exec.Command("sh", "-c", tc.body), "", tc.timeout, []string{"test"})
			if err == nil || !strings.Contains(out, "before-") {
				t.Fatalf("out=%q err=%v", out, err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("取消原因丢失: %v", err)
			}
			if tc.want == nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 7 {
					t.Fatalf("退出码丢失: %v", err)
				}
			}
		})
	}
}

func TestStreamCmdExitedParentDoesNotWaitForInheritedPipes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := streamCmd(ctx, exec.CommandContext(ctx, "sh", "-c", "sleep 10 & echo parent-exited"))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		for line := range stream.lines {
			b.WriteString(line)
		}
		done <- b.String()
	}()
	select {
	case output := <-done:
		if !strings.Contains(output, "parent-exited") || stream.err == nil {
			t.Fatalf("管道超期应保留输出并报明细: %q %v", output, stream.err)
		}
	case <-time.After(3 * time.Second):
		cancel()
		<-done
		t.Fatal("父进程已退出，孙进程持有日志管道让流一直挂起")
	}
}

func TestStreamCmdBackpressureReleasesReadersAfterParentExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	marker := filepath.Join(t.TempDir(), "exited")
	stream, err := streamCmd(ctx, exec.Command("sh", "-c", "i=0; while [ $i -lt 200 ]; do echo line; i=$((i+1)); done; touch '"+marker+"'"))
	if err != nil {
		t.Fatal(err)
	}
	waitForProbeMarker(t, marker)
	// 直接进程退出后不读取输出，模拟客户端背压超过管道排空期限。
	time.Sleep(2300 * time.Millisecond)
	count := 0
	for range stream.lines {
		count++
	}
	if count > 64 || !errors.Is(stream.err, exec.ErrWaitDelay) {
		t.Fatalf("背压未及时释放读取线程: lines=%d err=%v", count, stream.err)
	}
}

func TestStreamCmdPreservesAllOutputOnExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", "i=0; while [ $i -lt 4000 ]; do printf 'out-%s\\n' \"$i\"; printf 'err-%s\\n' \"$i\" >&2; i=$((i+1)); done")
	stream, err := streamCmd(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]int, 8000)
	for line := range stream.lines {
		seen[strings.TrimSuffix(line, "\n")]++
	}
	if stream.err != nil {
		t.Fatalf("成功命令排空输出失败: %v", stream.err)
	}
	for i := range 4000 {
		for _, prefix := range []string{"out", "err"} {
			line := fmt.Sprintf("%s-%d", prefix, i)
			if seen[line] != 1 {
				t.Fatalf("尾部输出截断或重复: %q 出现 %d 次", line, seen[line])
			}
		}
	}
	if len(seen) != 8000 {
		t.Fatalf("输出行数=%d，期望8000", len(seen))
	}
}

func TestRunCmdPreservesAllOutputOnExit(t *testing.T) {
	for _, exitCode := range []int{0, 7} {
		t.Run(fmt.Sprintf("exit-%d", exitCode), func(t *testing.T) {
			body := fmt.Sprintf(`i=0; while [ $i -lt 4000 ]; do printf 'out-%%s\n' "$i"; printf 'err-%%s\n' "$i" >&2; i=$((i+1)); done; exit %d`, exitCode)
			out, err := runCmd(context.Background(), exec.Command("sh", "-c", body), "", 5*time.Second, []string{"test"})
			if (err != nil) != (exitCode != 0) {
				t.Fatalf("exit=%d err=%v", exitCode, err)
			}
			seen := make(map[string]int)
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				seen[line]++
			}
			prefixes := []string{"out"}
			if exitCode != 0 {
				prefixes = append(prefixes, "err")
			}
			for i := range 4000 {
				for _, prefix := range prefixes {
					line := fmt.Sprintf("%s-%d", prefix, i)
					if seen[line] != 1 {
						t.Fatalf("输出截断或重复: %q 出现 %d 次", line, seen[line])
					}
				}
			}
		})
	}
}
