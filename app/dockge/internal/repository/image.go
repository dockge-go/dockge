// 镜像域：本地镜像列表、删除、拉取与未使用镜像清理。
// podman client 优先，不可用时降级 docker CLI。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
					ID:          truncateID(s.ID),
					Repo:        repo,
					Tag:         tag,
					SizeBytes:   s.Size,
					CreatedUnix: s.Created,
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
			CreatedAt    string `json:"CreatedAt"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse docker images output: %w", err)
		}
		created, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", item.CreatedAt)
		result = append(result, model.Image{
			ID:          strings.TrimPrefix(item.ID, "sha256:"),
			Repo:        item.Repository,
			Tag:         item.Tag,
			SizeBytes:   parseDockerSize(item.Size),
			CreatedUnix: created.Unix(),
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

// formatBytes 把字节数格式化为人类可读大小（1024 进制，供 stats 内存用量展示）。
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

// dockerSizeUnits 是 docker CLI 人类可读大小到字节数的乘数表：
// 十进制（kB/MB/GB）与二进制（KiB/MiB/GiB）并存。
var dockerSizeUnits = map[string]int64{
	"B": 1, "kB": 1_000, "KB": 1_000, "MB": 1_000_000, "GB": 1_000_000_000,
	"TB": 1_000_000_000_000, "PB": 1_000_000_000_000_000,
	"KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30, "TiB": 1 << 40, "PiB": 1 << 50,
}

// parseDockerSize 把 docker 输出的人类可读大小（如 "52.2MB"、"1.234kB"、"0B"）
// 解析为字节数；无法解析时返回 0。
func parseDockerSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// 分离数值与单位：数值部分以数字或小数点结尾
	i := len(s)
	for i > 0 && (s[i-1] == '.' || (s[i-1] >= '0' && s[i-1] <= '9')) {
		i--
	}
	valueStr, unit := s[:i], strings.TrimSpace(s[i:])
	if valueStr == "" {
		return 0
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0
	}
	mul, ok := dockerSizeUnits[unit]
	if !ok {
		return 0
	}
	return int64(value * float64(mul))
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
