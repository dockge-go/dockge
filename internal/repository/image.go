// 镜像管理：列表（docker images）、手动拉取与批量删除，经容器 CLI 子进程完成。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"dockge/internal/model"
)

// ImageRefPattern 校验镜像引用（CLI 参数），防止参数注入。
var ImageRefPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]*$`)

// imageItem 映射 `docker images --format json` 的一行（NDJSON 或数组）。
type imageItem struct {
	ID           string `json:"ID"`
	Repository   string `json:"Repository"`
	Tag          string `json:"Tag"`
	Size         string `json:"Size"`
	CreatedSince string `json:"CreatedSince"`
	Containers   string `json:"Containers"`
}

// ListImages 返回本机镜像列表；Containers 计数非零（含已停止容器）即视为使用中。
func (r *Repository) ListImages(ctx context.Context) ([]model.Image, error) {
	out, err := r.runDocker(ctx, composeOpTimeout, "images", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("docker images: %w", err)
	}
	items := make([]imageItem, 0)
	if strings.TrimSpace(out) != "" {
		// 兼容数组与 NDJSON 两种输出（同 compose ps）
		if err := json.Unmarshal([]byte(out), &items); err != nil {
			items = items[:0]
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var item imageItem
				if err := json.Unmarshal([]byte(line), &item); err != nil {
					return nil, fmt.Errorf("parse docker images output: %w", err)
				}
				items = append(items, item)
			}
		}
	}
	images := make([]model.Image, 0, len(items))
	for _, it := range items {
		images = append(images, model.Image{
			ID: it.ID, Repository: it.Repository, Tag: it.Tag,
			Size: it.Size, CreatedSince: it.CreatedSince,
			InUse: it.Containers != "" && it.Containers != "0",
		})
	}
	return images, nil
}

// DeleteImages 批量删除镜像（docker rmi），返回输出供前端展示；
// 使用中的镜像会被 docker 拒绝并体现在输出里。
func (r *Repository) DeleteImages(ctx context.Context, refs []string) (string, error) {
	args := append([]string{"rmi"}, refs...)
	return r.runDocker(ctx, composeOpTimeout, args...)
}

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
	for line := range ch {
		if err := filter.line(strings.TrimSuffix(line, "\n")); err != nil {
			return err
		}
	}
	if err := filter.flush(); err != nil {
		return err
	}
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("docker pull 超时（%s）", composeUpdateTimeout)
		}
		return fmt.Errorf("docker pull 失败（退出码 %d）", code)
	}
	return nil
}
