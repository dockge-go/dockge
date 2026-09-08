// 数据卷域：卷列表、删除与未使用卷清理。
// podman client 优先，不可用时降级 docker CLI。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.podman.io/podman/v6/pkg/bindings/volumes"

	"dockge/app/dockge/internal/model"
)

// DockerVolumes 返回本地全部数据卷。podman client 优先，不可用时降级 docker CLI。
func (r *Repository) DockerVolumes(ctx context.Context) ([]model.Volume, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		reports, err := volumes.List(ctx, &volumes.ListOptions{})
		if err == nil {
			result := make([]model.Volume, 0, len(reports))
			for _, v := range reports {
				result = append(result, model.Volume{Name: v.Name, Driver: v.Driver})
			}
			return result, nil
		}
		r.logger.Warn().Err(err).Msg("podman volume list failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, 30*time.Second, "volume", "ls", "--format", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("docker volume ls: %w", err)
	}
	result := make([]model.Volume, 0)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item struct {
			Name   string `json:"Name"`
			Driver string `json:"Driver"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse docker volume ls output: %w", err)
		}
		result = append(result, model.Volume{Name: item.Name, Driver: item.Driver})
	}
	return result, nil
}

// RemoveVolume 删除指定数据卷。podman client 优先，不可用时降级 docker CLI。
func (r *Repository) RemoveVolume(ctx context.Context, name string) error {
	if r.podman != nil && r.podman.IsAvailable() {
		if err := volumes.Remove(ctx, name, &volumes.RemoveOptions{Force: boolPtr(true)}); err == nil {
			return nil
		} else {
			r.logger.Warn().Msg("podman volume remove failed, falling back to docker CLI")
		}
	}
	_, err := runDocker(ctx, 30*time.Second, "volume", "rm", "-f", name)
	return err
}

// PruneVolumes 清理未被任何容器使用的数据卷（docker volume prune -af 语义）。
func (r *Repository) PruneVolumes(ctx context.Context) (string, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		reports, err := volumes.Prune(ctx, &volumes.PruneOptions{})
		if err == nil {
			return fmt.Sprintf("已清理 %d 个未使用数据卷", len(reports)), nil
		}
		r.logger.Warn().Msg("podman volume prune failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, 30*time.Second, "volume", "prune", "-a", "-f")
	return out, err
}
