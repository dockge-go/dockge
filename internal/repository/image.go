// 镜像管理：列表（docker images）、手动拉取与批量删除，经容器 CLI 子进程完成。
package repository

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// ImageRefPattern 校验镜像引用（CLI 参数），防止参数注入。
var ImageRefPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]*$`)

// PullImageStream 流式拉取镜像：输出经 progressFilter 折叠进度 tick 后写入 w，
// 语义与 StackOpStream 一致（超时放宽：拉取可能较慢）。
func (r *Repository) PullImageStream(ctx context.Context, w io.Writer, ref string) error {
	ctx, cancel := context.WithTimeout(ctx, composeUpdateTimeout)
	defer cancel()
	cmd, err := r.runtime.Command(ctx, "pull", ref)
	if err != nil {
		return err
	}
	ch, err := streamCmd(ctx, cmd)
	if err != nil {
		return err
	}
	filter := &progressFilter{w: w}
	for line := range ch.lines {
		if err := filter.line(strings.TrimSuffix(line, "\n")); err != nil {
			return err
		}
	}
	if err := filter.flush(); err != nil {
		return err
	}
	if ch.err != nil {
		code := cmd.ProcessState.ExitCode()
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("docker pull 超时（%s）", composeUpdateTimeout)
		}
		return fmt.Errorf("docker pull 失败（退出码 %d）: %w", code, ch.err)
	}
	return nil
}
