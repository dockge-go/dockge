// Package repository 是 dockge 的数据访问层：对接 docker/podman 引擎、
// bbolt 存储与 /proc 主机统计。本文件承载 docker/podman CLI 执行机制。
package repository

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ContainerIDPattern 校验容器 ID/名称（CLI 参数），防止参数注入。
var ContainerIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// runDocker 执行容器 CLI 并返回标准输出（stderr 拼入错误信息）。
func (r *Repository) runDocker(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	return r.runDockerIn(ctx, "", timeout, args...)
}

// runDockerIn 在指定工作目录执行容器 CLI（docker/podman/nerdctl，见 Runtime）；
// 超时或非零退出都返回错误，错误信息中携带命令输出，便于前端直接展示报错细节。
func (r *Repository) runDockerIn(ctx context.Context, dir string, timeout time.Duration, args ...string) (string, error) {
	cmd, err := r.runtime.Command(ctx, args...)
	if err != nil {
		return "", err
	}
	return runCmd(ctx, cmd, dir, timeout, args)
}

// runComposeIn 执行 compose 命令（`<cli> compose` 或独立 compose 命令如 podman-compose）。
func (r *Repository) runComposeIn(ctx context.Context, dir string, timeout time.Duration, args ...string) (string, error) {
	cmd, err := r.runtime.ComposeCommand(ctx, args...)
	if err != nil {
		return "", err
	}
	return runCmd(ctx, cmd, dir, timeout, args)
}

// runCmd 统一的子进程执行：带超时、收集输出、错误信息携带命令与输出。
func runCmd(ctx context.Context, cmd *exec.Cmd, dir string, timeout time.Duration, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	old := cmd
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	cmd.Env, cmd.Dir = old.Env, old.Dir
	group := newCommandProcessGroup(cmd)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err == nil {
		err = group.finish()
	}
	if err != nil {
		msg := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		name := filepath.Base(cmd.Path)
		cause := errors.Join(err, ctx.Err())
		detail := ""
		if msg != "" {
			detail = ": " + msg
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return msg, fmt.Errorf("%s %s: 超时（命令上限 %s）: %w%s", name, strings.Join(args, " "), timeout, cause, detail)
		}
		return msg, fmt.Errorf("%s %s: %w%s", name, strings.Join(args, " "), cause, detail)
	}
	return stdout.String(), nil
}

// shortID 截断容器 ID 便于展示。
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// streamCmd 启动命令并逐行转发 stdout/stderr 到通道。
// 两个流并发读取：docker logs -f 的 stdout 永不 EOF，
// 若串行合并（MultiReader），容器 stderr 侧的日志将永远读不到。
type commandStream struct {
	lines <-chan string
	err   error // 仅在 lines 关闭后读取
}

func streamCmd(ctx context.Context, cmd *exec.Cmd) (*commandStream, error) {
	// 自持管道：Wait 与读取并行，避免 StdoutPipe 被 Wait 提前关闭而截断末尾输出。
	outPipe, outWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	errPipe, errWriter, err := os.Pipe()
	if err != nil {
		outPipe.Close()
		outWriter.Close()
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = outWriter, errWriter
	group := newCommandProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		outPipe.Close()
		errPipe.Close()
		outWriter.Close()
		errWriter.Close()
		return nil, err
	}
	// 父进程不保留写端，否则即便所有子进程退出也收不到 EOF。
	outWriter.Close()
	errWriter.Close()
	readCtx, stopReading := context.WithCancel(ctx)
	ch := make(chan string, 64)
	result := &commandStream{lines: ch}
	var readMu sync.Mutex
	var readErr error
	var wg sync.WaitGroup
	scan := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-readCtx.Done():
				return
			case ch <- scanner.Text() + "\n":
			}
		}
		if err := scanner.Err(); err != nil && readCtx.Err() == nil {
			readMu.Lock()
			readErr = errors.Join(readErr, fmt.Errorf("读取命令输出失败（单行限制 1 MiB）: %w", err))
			readMu.Unlock()
			group.kill()
		}
	}
	wg.Add(2)
	go scan(outPipe)
	go scan(errPipe)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	go func() {
		defer stopReading()
		defer outPipe.Close()
		defer errPipe.Close()
		exited := make(chan error, 1)
		go func() { exited <- group.waitForExit() }()
		var waitErr error
		select {
		case <-ctx.Done():
			group.kill()
			outPipe.Close()
			errPipe.Close()
			waitErr = <-exited
		case waitErr = <-exited:
		}
		// 直接进程退出后仍给末尾输出留出读取时间；孙进程不能无限占用管道。
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-done:
		case <-ctx.Done():
			group.kill()
			outPipe.Close()
			errPipe.Close()
			<-done
		case <-timer.C:
			// 管道关闭不会解除向满通道发送的阻塞，需独立停止读取者。
			stopReading()
			group.kill()
			outPipe.Close()
			errPipe.Close()
			<-done
			waitErr = errors.Join(waitErr, fmt.Errorf("命令进程已退出但输出管道在 2s 内未结束（可能由子进程持有）: %w", exec.ErrWaitDelay))
		}
		// EOF 只表示描述符已关闭；组内仍可能有重定向输出的后台后代。
		// Linux 此时组长尚未收尸，PGID 不会重用；其他平台已禁用迟到清理。
		group.kill()
		waitErr = errors.Join(waitErr, group.wait())
		result.err = errors.Join(readErr, waitErr, ctx.Err())
		close(ch)
	}()
	return result, nil
}

// commandProcessGroup 将取消和最终清理串成一次操作。Linux 先杀组再收尸；
// 不支持保留退出状态的平台只通过 os.Process 取消，不在收尸后按数字 PGID 杀组。
type commandProcessGroup struct {
	cmd    *exec.Cmd
	once   sync.Once
	waited bool // 仅等待协程写入，调用者收到退出结果后读取。
}

func newCommandProcessGroup(cmd *exec.Cmd) *commandProcessGroup {
	return configureCommandProcessGroup(cmd, false)
}

func configureCommandProcessGroup(cmd *exec.Cmd, usePTY bool) *commandProcessGroup {
	if !usePTY {
		setProcessGroup(cmd)
	}
	group := &commandProcessGroup{cmd: cmd}
	// 普通 exec.Command 不允许设置 Cancel；CommandContext 则必须覆盖默认的
	// 单进程取消，避免其与最终整组清理独立触发。
	if cmd.Cancel != nil {
		cmd.Cancel = func() error { group.kill(); return nil }
	}
	cmd.WaitDelay = 2 * time.Second
	return group
}

func (g *commandProcessGroup) kill() {
	g.once.Do(func() { killCommandProcessGroup(g.cmd) })
}

// waitForExit 在支持非收尸等待的平台保留组长 PID，直到最终清理完成。
func (g *commandProcessGroup) waitForExit() error {
	waited, err := waitForProcessExit(g.cmd)
	g.waited = waited
	if waited || errors.Is(err, syscall.ECHILD) {
		// 平台降级或外部已收尸时禁止迟到的数字 PID 清理。
		g.once.Do(func() {})
	}
	return err
}

func (g *commandProcessGroup) wait() error {
	if g.waited {
		return nil
	}
	return g.cmd.Wait()
}

func (g *commandProcessGroup) finish() error {
	err := g.waitForExit()
	g.kill()
	return errors.Join(err, g.wait())
}

// ConfigureStreamingCommand 统一日志、终端与自检的安全退出流程。
// Start 成功后必须调用返回的等待函数而非 cmd.Wait；它先清组再收尸，
// 并使完成后的重复 Cancel 无害。PTY 自建 session，不能同时 Setpgid。
func ConfigureStreamingCommand(cmd *exec.Cmd, usePTY bool) func() error {
	return configureCommandProcessGroup(cmd, usePTY).finish
}
