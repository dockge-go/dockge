package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// preflightTimeout 自检超时：守护进程卡死时不应拖住启动与健康检查。
const preflightTimeout = 5 * time.Second

// RuntimeStatus 是容器运行时自检结果，随 /v1/health 暴露：用户遇到
// 「面板能开、栈操作全失败」（最常见是漏挂 docker socket）时可据此自助排查。
type RuntimeStatus struct {
	CLI     string       `json:"cli"`
	Compose string       `json:"compose"`
	Ready   bool         `json:"ready"`
	Error   string       `json:"error,omitempty"`
	Stacks  StacksStatus `json:"stacks"`
}

// StacksStatus 描述栈目录及其持久性：Mounted=false 表示目录不在宿主挂载的
// 卷/绑定目录下，栈只写在容器可写层，容器重建即丢失。
type StacksStatus struct {
	Path     string `json:"path"`
	Mounted  bool   `json:"mounted"`
	Writable bool   `json:"writable"`
	Error    string `json:"error,omitempty"`
}

// Preflight 短暂缓存自检，避免永久保留启动时的故障。
// 只报告不阻断：运行时不可用时面板的登录/设置/浏览仍应可用，呈现方式交由调用方。
//
// 必须用 info 而非 compose version：后者只打印 CLI 版本、不接触守护进程，
// 漏挂 docker socket 时依然成功，会漏报最常见的部署事故。
func (r *Repository) Preflight(ctx context.Context) RuntimeStatus {
	if err := ctx.Err(); err != nil {
		return r.canceledPreflight(err)
	}
	r.preflightMu.Lock()
	if !r.preflightAt.IsZero() && time.Since(r.preflightAt) < time.Second {
		status := r.preflightStatus
		r.preflightMu.Unlock()
		return status
	}
	done := r.preflightDone
	if done == nil {
		done = make(chan struct{})
		r.preflightDone = done
		// 自检属于共享基础设施，不随第一个订阅者断线取消；最多同时运行一次且有独立期限。
		go func() {
			probeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), preflightTimeout)
			defer cancel()
			status := r.probeRuntime(probeCtx)
			r.preflightMu.Lock()
			r.preflightStatus, r.preflightAt = status, time.Now()
			r.preflightDone = nil
			close(done)
			r.preflightMu.Unlock()
		}()
	}
	r.preflightMu.Unlock()
	select {
	case <-ctx.Done():
		return r.canceledPreflight(ctx.Err())
	case <-done:
		if err := ctx.Err(); err != nil {
			return r.canceledPreflight(err)
		}
		r.preflightMu.Lock()
		status := r.preflightStatus
		r.preflightMu.Unlock()
		return status
	}
}

func (r *Repository) canceledPreflight(err error) RuntimeStatus {
	rt := r.runtime.resolved()
	return RuntimeStatus{CLI: rt.bin, Compose: rt.composeBin, Error: fmt.Sprintf("运行时自检请求已结束，未判定运行时故障: %v", err), Stacks: StacksStatus{Path: r.stacksDir}}
}

func (r *Repository) probeRuntime(ctx context.Context) RuntimeStatus {
	rt := r.runtime.resolved()
	status := RuntimeStatus{CLI: rt.bin, Compose: rt.composeBin, Stacks: StacksStatus{Path: r.stacksDir, Mounted: stacksPersistent(r.stacksDir)}}
	probe, err := os.CreateTemp(r.stacksDir, ".dockge-probe-*")
	if err != nil {
		status.Stacks.Error = fmt.Sprintf("栈目录 %s 不可写: %v", r.stacksDir, err)
	} else {
		path := probe.Name()
		err = probe.Close()
		if removeErr := os.Remove(path); err == nil {
			err = removeErr
		}
		if err != nil {
			status.Stacks.Error = fmt.Sprintf("栈目录 %s 写入自检清理失败: %v", r.stacksDir, err)
		} else {
			status.Stacks.Writable = true
		}
	}
	cmd, err := rt.Command(ctx, "info")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	wait := ConfigureStreamingCommand(cmd, false)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = io.Discard, &stderr
	err = cmd.Start()
	if err == nil {
		err = wait()
	}
	if err != nil {
		cause := errors.Join(err, ctx.Err())
		status.Error = fmt.Sprintf("%s info 自检失败（期限 %s）: %v", rt.bin, preflightTimeout, cause)
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			status.Error += ": " + detail
		}
	} else {
		status.Ready = true
	}
	return status
}
