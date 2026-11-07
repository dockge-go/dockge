package repository

import (
	"bytes"
	"context"
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
	r.preflightMu.Lock()
	defer r.preflightMu.Unlock()
	if !r.preflightAt.IsZero() && time.Since(r.preflightAt) < time.Second {
		return r.preflightStatus
	}
	{
		status := RuntimeStatus{
			CLI:     r.runtime.bin,
			Compose: r.runtime.composeBin,
			Stacks:  StacksStatus{Path: r.stacksDir, Mounted: stacksPersistent(r.stacksDir)},
		}
		probe, e := os.CreateTemp(r.stacksDir, ".dockge-probe-*")
		if e != nil {
			status.Stacks.Error = fmt.Sprintf("栈目录 %s 不可写: %v", r.stacksDir, e)
		} else {
			path := probe.Name()
			e = probe.Close()
			if removeErr := os.Remove(path); e == nil {
				e = removeErr
			}
			if e != nil {
				status.Stacks.Error = fmt.Sprintf("栈目录 %s 写入自检清理失败: %v", r.stacksDir, e)
			} else {
				status.Stacks.Writable = true
			}
		}
		probeCtx, cancel := context.WithTimeout(ctx, preflightTimeout)
		defer cancel()
		if cmd, err := r.runtime.Command(probeCtx, "info"); err != nil {
			status.Error = err.Error()
		} else {
			// stdout 只有 "Client:" 这类分段标题，真正的失败原因在 stderr
			var stderr bytes.Buffer
			cmd.Stdout = io.Discard
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				status.Error = strings.TrimSpace(stderr.String())
				if status.Error == "" {
					status.Error = err.Error()
				}
			} else {
				status.Ready = true
			}
		}
		r.preflightStatus = status
		r.preflightAt = time.Now()
	}
	return r.preflightStatus
}
