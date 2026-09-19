package repository

import (
	"bytes"
	"context"
	"io"
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
	Path    string `json:"path"`
	Mounted bool   `json:"mounted"`
}

// Preflight 首次调用时探测运行时（CLI info）并缓存结果。
// 只报告不阻断：运行时不可用时面板的登录/设置/浏览仍应可用，呈现方式交由调用方。
//
// 必须用 info 而非 compose version：后者只打印 CLI 版本、不接触守护进程，
// 漏挂 docker socket 时依然成功，会漏报最常见的部署事故。
func (r *Repository) Preflight(ctx context.Context) RuntimeStatus {
	r.preflightOnce.Do(func() {
		status := RuntimeStatus{
			CLI:     r.runtime.bin,
			Compose: r.runtime.composeBin,
			Stacks:  StacksStatus{Path: r.stacksDir, Mounted: stacksPersistent(r.stacksDir)},
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
				status.Error = firstLine(stderr.Bytes())
				if status.Error == "" {
					status.Error = err.Error()
				}
			} else {
				status.Ready = true
			}
		}
		r.preflightStatus = status
	})
	return r.preflightStatus
}

// firstLine 取输出的首个非空行：CLI 报错首行（如 cannot connect to the Docker daemon）
// 远比 "exit status 1" 有指导性。
func firstLine(out []byte) string {
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
