// 镜像域：本地镜像列表、删除、拉取与未使用镜像清理。
// podman client 优先，不可用时降级 docker CLI。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.podman.io/podman/v6/pkg/bindings/images"

	"dockge/app/dockge/internal/model"
)

// DockerImages 返回本地存储的全部镜像。podman client 优先，不可用时降级 docker CLI。
func (r *Repository) DockerImages(ctx context.Context) ([]model.Image, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		summaries, err := images.List(ctx, &images.ListOptions{})
		if err == nil {
			result := make([]model.Image, 0, len(summaries))
			for _, s := range summaries {
				repo, tag := "", "<none>"
				if len(s.RepoTags) > 0 {
					repo, tag = splitRepoTag(s.RepoTags[0])
				} else if len(s.Names) > 0 {
					repo, tag = splitRepoTag(s.Names[0])
				}
				result = append(result, model.Image{
					ID:      truncateID(s.ID),
					Repo:    repo,
					Tag:     tag,
					Size:    formatBytes(float64(s.Size)),
					Created: humanSince(s.Created),
				})
			}
			return result, nil
		}
		r.logger.Warn().Err(err).Msg("podman image list failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, 30*time.Second, "images", "--format", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("docker images: %w", err)
	}
	result := make([]model.Image, 0)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item struct {
			ID           string `json:"ID"`
			Repository   string `json:"Repository"`
			Tag          string `json:"Tag"`
			Size         string `json:"Size"`
			CreatedSince string `json:"CreatedSince"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse docker images output: %w", err)
		}
		result = append(result, model.Image{
			ID:      strings.TrimPrefix(item.ID, "sha256:"),
			Repo:    item.Repository,
			Tag:     item.Tag,
			Size:    item.Size,
			Created: item.CreatedSince,
		})
	}
	return result, nil
}

// RemoveImage 删除本地镜像。podman client 优先，不可用时降级 docker CLI。
func (r *Repository) RemoveImage(ctx context.Context, id string) error {
	if r.podman != nil && r.podman.IsAvailable() {
		if _, errs := images.Remove(ctx, []string{id}, &images.RemoveOptions{Force: boolPtr(true)}); len(errs) == 0 {
			return nil
		} else {
			for _, e := range errs {
				r.logger.Warn().Err(e).Msg("podman image remove failed, falling back to docker CLI")
			}
		}
	}
	_, err := runDocker(ctx, composeOpTimeout, "rmi", "-f", id)
	return err
}

// PullImage 拉取镜像（耗时操作，超时放宽到 10 分钟）。
// podman client 优先，不可用时降级 docker CLI，返回组合输出。
func (r *Repository) PullImage(ctx context.Context, reference string) (string, error) {
	if !containerIDPattern.MatchString(reference) || strings.Contains(reference, " ") {
		return "", fmt.Errorf("非法镜像引用: %s", reference)
	}
	pullCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if r.podman != nil && r.podman.IsAvailable() {
		if refs, err := images.Pull(pullCtx, reference, &images.PullOptions{}); err == nil {
			return "已拉取: " + strings.Join(refs, ", "), nil
		} else {
			r.logger.Warn().Err(err).Msg("podman pull failed, falling back to docker CLI")
		}
	}
	out, err := runDocker(pullCtx, 10*time.Minute, "pull", reference)
	if err != nil {
		return out, err
	}
	return out, nil
}

// splitRepoTag 把 "nginx:alpine" 拆成仓库名与标签（跳过 registry 端口冒号）。
func splitRepoTag(repoTag string) (string, string) {
	idx := strings.LastIndex(repoTag, ":")
	if idx < 0 || strings.Contains(repoTag[idx+1:], "/") {
		return repoTag, "latest"
	}
	return repoTag[:idx], repoTag[idx+1:]
}

// formatBytes 把字节数格式化为人类可读大小。
func formatBytes(b float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	for _, u := range units {
		if b < 1024 {
			return fmt.Sprintf("%.1f%s", b, u)
		}
		b /= 1024
	}
	return fmt.Sprintf("%.1fPB", b)
}

// humanSince 把 unix 秒转换为 "N days ago" 式的人类可读时间。
func humanSince(unixSec int64) string {
	d := time.Since(time.Unix(unixSec, 0))
	switch {
	case d.Hours() < 1:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d.Hours() < 24:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}

// PruneImages 清理未被任何容器引用的镜像（docker image prune -af 语义）。
func (r *Repository) PruneImages(ctx context.Context) (string, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		reports, err := images.Prune(ctx, &images.PruneOptions{All: boolPtr(true)})
		if err == nil {
			return fmt.Sprintf("已清理 %d 个未使用镜像", len(reports)), nil
		}
		r.logger.Warn().Err(err).Msg("podman image prune failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, composeOpTimeout, "image", "prune", "-a", "-f")
	return out, err
}
