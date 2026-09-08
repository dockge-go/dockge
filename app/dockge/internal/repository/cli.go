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
	"regexp"
	"strings"
	"sync"
	"time"
)

// containerIDPattern 校验容器 ID/名称/镜像引用（CLI 参数），防止参数注入。
var containerIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// runDocker 执行 docker CLI 并返回标准输出（stderr 拼入错误信息）。
func runDocker(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	return runDockerIn(ctx, "", timeout, args...)
}

// runDockerIn 在指定工作目录执行 docker CLI；超时或非零退出都返回错误，
// 错误信息中携带命令输出，便于前端直接展示 compose 的报错细节。
func runDockerIn(ctx context.Context, dir string, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
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
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return msg, fmt.Errorf("docker %s: 超时（%s）", strings.Join(args, " "), timeout)
		}
		if msg != "" {
			return msg, fmt.Errorf("docker %s: %s", args[0], msg)
		}
		return msg, fmt.Errorf("docker %s: %w", args[0], err)
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
	go func() {
		wg.Wait()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		close(ch)
	}()
	return ch, nil
}
