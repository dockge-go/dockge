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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		name := filepath.Base(cmd.Path)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return msg, fmt.Errorf("%s %s: 超时（%s）", name, strings.Join(args, " "), timeout)
		}
		if msg != "" {
			return msg, fmt.Errorf("%s %s: %s", name, args[0], msg)
		}
		return msg, fmt.Errorf("%s %s: %w", name, args[0], err)
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
func streamCmd(ctx context.Context, cmd *exec.Cmd) (<-chan string, error) {
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		outPipe.Close()
		return nil, err
	}
	// 独立进程组：孙进程（compose 插件等）继承管道，超时必须整组终止，
	// 否则 scanner 等 EOF 永不返回（平台差异见 process_unix.go / process_windows.go）。
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		outPipe.Close()
		errPipe.Close()
		return nil, err
	}
	ch := make(chan string, 64)
	var wg sync.WaitGroup
	scan := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- scanner.Text() + "\n":
			}
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
		select {
		case <-ctx.Done():
			killProcessGroup(cmd)
		case <-done:
		}
		// Wait 收尸设置 ProcessState；缺失时 ExitCode() 恒为 -1，
		// 流式栈操作会被误报「失败」。
		_ = cmd.Wait()
		close(ch)
	}()
	return ch, nil
}
