//go:build linux

package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// 真 shell 启动后台后代，父进程等测试取得稳定的 pidfd 后才退出。
// 后代主动关闭日志描述符，确保 EOF 不能被误当作整个进程组已退出。
func TestCLICleansBackgroundProcessGroup(t *testing.T) {
	for _, mode := range []string{"run", "stream", "configured"} {
		for _, exitCode := range []int{0, 7} {
			for _, closed := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/exit-%d/closed-output-%t", mode, exitCode, closed), func(t *testing.T) {
					probe := newCLIChildProbe(t, closed)
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					cmd := probe.command(exitCode)
					done := make(chan cliProbeResult, 1)
					go func() { done <- collectCLIProbe(ctx, mode, cmd) }()
					probe.captureChild(t)
					probe.releaseParent(t)
					got := awaitCLIProbe(t, done)
					if !strings.Contains(got.output, "parent-output") {
						t.Fatalf("父进程尾部输出丢失: %q (%v)", got.output, got.err)
					}
					if exitCode == 0 {
						if mode == "stream" && !closed {
							if !errors.Is(got.err, exec.ErrWaitDelay) {
								t.Fatalf("继承输出管道超期原因丢失: %v", got.err)
							}
						} else if got.err != nil {
							t.Fatalf("正常退出被误报失败: %v", got.err)
						}
					} else {
						var exit *exec.ExitError
						if !errors.As(got.err, &exit) || exit.ExitCode() != exitCode || !strings.Contains(got.output, "parent-error") {
							t.Fatalf("退出码或错误输出丢失: output=%q err=%v", got.output, got.err)
						}
					}
					probe.assertStopped(t)
				})
			}
		}
	}
}

func TestCLICancelCleansBackgroundProcessGroup(t *testing.T) {
	for _, mode := range []string{"run", "stream", "configured"} {
		for _, commandContext := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/command-context-%t", mode, commandContext), func(t *testing.T) {
				probe := newCLIChildProbe(t, true)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				cmd := probe.command(0)
				if commandContext {
					cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
				}
				done := make(chan cliProbeResult, 1)
				go func() { done <- collectCLIProbe(ctx, mode, cmd) }()
				probe.captureChild(t)
				// 父进程尚未退出，取消须同时终止不持有管道的后代。
				cancel()
				got := awaitCLIProbe(t, done)
				if !errors.Is(got.err, context.Canceled) {
					t.Fatalf("取消原因丢失: %v", got.err)
				}
				probe.assertStopped(t)
				if commandContext && mode == "stream" {
					// Wait 已收尸后重复调用 Cancel 必须是无害的，不能再按 PID 杀组。
					if err := cmd.Cancel(); err != nil {
						t.Fatalf("迟到取消: %v", err)
					}
				}
			})
		}
	}
}

func TestCLIExitCancelRaceCleansBackgroundProcessGroup(t *testing.T) {
	for _, mode := range []string{"run", "stream", "configured"} {
		t.Run(mode, func(t *testing.T) {
			for range 20 {
				probe := newCLIChildProbe(t, true)
				ctx, cancel := context.WithCancel(context.Background())
				t.Cleanup(cancel)
				cmd := probe.command(0)
				cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
				done := make(chan cliProbeResult, 1)
				go func() { done <- collectCLIProbe(ctx, mode, cmd) }()
				probe.captureChild(t)
				probe.releaseParent(t)
				canceled := make(chan struct{})
				go func() {
					cancel()
					close(canceled)
				}()
				got := awaitCLIProbe(t, done)
				<-canceled
				if got.err != nil && !errors.Is(got.err, context.Canceled) {
					t.Fatalf("退出/取消竞态产生非预期错误: %v", got.err)
				}
				probe.assertStopped(t)
			}
		})
	}
}

func TestStreamCmdReadFailureCleansBackgroundProcessGroup(t *testing.T) {
	probe := newCLIChildProbe(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := probe.command(0)
	cmd.Args[2] += `
head -c 1100000 /dev/zero | tr '\000' x; printf '\n'`
	// 用超长输出替换正常退出，让 Scanner 的错误路径触发整组清理。
	cmd.Args[2] = strings.Replace(cmd.Args[2], `exit "$5"`, "", 1)
	done := make(chan cliProbeResult, 1)
	go func() { done <- collectCLIProbe(ctx, "stream", cmd) }()
	probe.captureChild(t)
	probe.releaseParent(t)
	got := awaitCLIProbe(t, done)
	if got.err == nil || !strings.Contains(got.err.Error(), "单行限制 1 MiB") {
		t.Fatalf("读取错误丢失: %v", got.err)
	}
	probe.assertStopped(t)
}

func TestStreamCmdBackpressureCleansBackgroundProcessGroup(t *testing.T) {
	probe := newCLIChildProbe(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := probe.command(0)
	cmd.Args[2] = strings.Replace(cmd.Args[2], `printf 'parent-output\n'`, `i=0; while [ $i -lt 200 ]; do echo line; i=$((i+1)); done`, 1)
	cmd.Args[2] = strings.Replace(cmd.Args[2], `exit "$5"`, `echo exited > "$3.exited"; exit "$5"`, 1)
	stream, err := streamCmd(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	probe.captureChild(t)
	probe.releaseParent(t)
	// 不消费日志，等超期释放背压后再读取；管道已 EOF 仍有后台后代。
	waitForProbeMarker(t, probe.releaseFile+".exited")
	time.Sleep(2300 * time.Millisecond)
	for range stream.lines {
	}
	if !errors.Is(stream.err, exec.ErrWaitDelay) {
		t.Fatalf("背压超期原因丢失: %v", stream.err)
	}
	probe.assertStopped(t)
}

func TestStreamCmdKeepsGroupLeaderUntilCleanup(t *testing.T) {
	probe := newCLIChildProbe(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := probe.command(0)
	stream, err := streamCmd(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	probe.captureChild(t)
	probe.releaseParent(t)
	// 在继承管道的排空宽限期内，已退出的组长必须仍是未收尸的原进程。
	deadline := time.Now().Add(time.Second)
	for {
		state, _, err := cliProcessIdentity(cmd.Process.Pid)
		if err != nil {
			t.Fatalf("整组清理前组长已收尸，PID 可被重用: %v", err)
		}
		if state == "Z" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("父进程未退出")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	for range stream.lines {
	}
	probe.assertStopped(t)
	if cmd.ProcessState == nil {
		t.Fatal("清理结束后组长未收尸")
	}
}

type cliProbeResult struct {
	output string
	err    error
}

func collectCLIProbe(ctx context.Context, mode string, cmd *exec.Cmd) cliProbeResult {
	if mode == "configured" {
		var stdout, stderr bytes.Buffer
		cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		wait := ConfigureStreamingCommand(cmd, false)
		err := cmd.Start()
		if err == nil {
			err = wait()
			// 终端结束后的延迟 Cancel 必须无害。
			_ = cmd.Cancel()
		}
		return cliProbeResult{stdout.String() + stderr.String(), errors.Join(err, ctx.Err())}
	}
	if mode == "run" {
		out, err := runCmd(ctx, cmd, "", 5*time.Second, []string{"test"})
		return cliProbeResult{out, err}
	}
	stream, err := streamCmd(ctx, cmd)
	if err != nil {
		return cliProbeResult{err: err}
	}
	var output strings.Builder
	for line := range stream.lines {
		output.WriteString(line)
	}
	return cliProbeResult{output.String(), stream.err}
}

func awaitCLIProbe(t *testing.T, done <-chan cliProbeResult) cliProbeResult {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(4 * time.Second):
		t.Fatal("CLI 调用未在进程退出/取消后有界结束")
		return cliProbeResult{}
	}
}

type cliChildProbe struct {
	pidFile, readyFile, releaseFile string
	closeOutput                     bool
	pid                             int
	start                           string
}

func newCLIChildProbe(t *testing.T, closeOutput bool) *cliChildProbe {
	t.Helper()
	dir := t.TempDir()
	return &cliChildProbe{
		pidFile: filepath.Join(dir, "child.pid"), readyFile: filepath.Join(dir, "child.ready"),
		releaseFile: filepath.Join(dir, "parent.release"), closeOutput: closeOutput,
	}
}

func (p *cliChildProbe) command(exitCode int) *exec.Cmd {
	child := `echo ready > "$1"; exec sleep 30`
	if p.closeOutput {
		child = `exec 1>&- 2>&-; ` + child
	}
	body := `sh -c "$4" child "$2" &
child=$!
printf '%s\n' "$child" > "$1"
while [ ! -f "$3" ]; do sleep 0.01; done
printf 'parent-output\n'
printf 'parent-error\n' >&2
exit "$5"`
	return exec.Command("sh", "-c", body, "parent", p.pidFile, p.readyFile, p.releaseFile, child, strconv.Itoa(exitCode))
}

func (p *cliChildProbe) captureChild(t *testing.T) {
	t.Helper()
	waitForProbeMarker(t, p.pidFile)
	waitForProbeMarker(t, p.readyFile)
	data, err := os.ReadFile(p.pidFile)
	if err != nil {
		t.Fatal(err)
	}
	p.pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	fd, err := unix.PidfdOpen(p.pid, 0)
	if err != nil {
		t.Fatalf("取得测试后代的稳定进程句柄: %v", err)
	}
	// 测试失败也只清理原进程；不能按可能已重用的数字 PID 发信号。
	t.Cleanup(func() {
		_ = unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0)
		_ = unix.Close(fd)
	})
	_, p.start, err = cliProcessIdentity(p.pid)
	if err != nil {
		t.Fatal(err)
	}
}

func (p *cliChildProbe) releaseParent(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(p.releaseFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (p *cliChildProbe) assertStopped(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		state, start, err := cliProcessIdentity(p.pid)
		if os.IsNotExist(err) || errors.Is(err, unix.ESRCH) || (err == nil && (start != p.start || state == "Z" || state == "X")) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("CLI 返回后后台后代仍在运行: pid=%d state=%s", p.pid, state)
		}
		time.Sleep(time.Millisecond)
	}
}

func cliProcessIdentity(pid int) (state, start string, err error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", "", err
	}
	// comm 可包含空格和括号，使用最后一个 ')' 后的字段。
	fields := strings.Fields(string(data)[strings.LastIndexByte(string(data), ')')+1:])
	if len(fields) < 20 {
		return "", "", fmt.Errorf("不完整的进程状态: %q", data)
	}
	return fields[0], fields[19], nil
}
