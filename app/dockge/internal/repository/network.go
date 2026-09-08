// 网络域：网络列表、详情、删除与未使用网络清理。
// podman client 优先，不可用时降级 docker CLI。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	libnetworkTypes "go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/pkg/bindings/network"
)

// DockerNetworks 返回本机全部 docker/podman 网络名称。
// podman client 优先，不可用时降级 docker CLI。
func (r *Repository) DockerNetworks(ctx context.Context) ([]string, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		nets, err := network.List(ctx, &network.ListOptions{})
		if err == nil {
			names := make([]string, 0, len(nets))
			for _, n := range nets {
				names = append(names, n.Name)
			}
			sort.Strings(names)
			return names, nil
		}
		r.logger.Warn().Err(err).Msg("podman network list failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, 30*time.Second, "network", "ls", "--format", "{{.Name}}")
	if err != nil {
		return nil, fmt.Errorf("docker network ls: %w", err)
	}
	names := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "NETWORK" {
			names = append(names, line)
		}
	}
	return names, nil
}

// RemoveNetwork 删除指定网络。podman client 优先，不可用时降级 docker CLI。
// 内置网络（bridge/host/none）由引擎侧拒绝删除，错误原样返回。
func (r *Repository) RemoveNetwork(ctx context.Context, name string) error {
	if r.podman != nil && r.podman.IsAvailable() {
		if _, err := network.Remove(ctx, name, &network.RemoveOptions{}); err == nil {
			return nil
		} else {
			r.logger.Warn().Err(err).Msg("podman network remove failed, falling back to docker CLI")
		}
	}
	_, err := runDocker(ctx, 30*time.Second, "network", "rm", name)
	return err
}

// NetworkInspect 返回指定网络的详细信息。
func (r *Repository) NetworkInspect(ctx context.Context, name string) (map[string]any, error) {
	out, err := runDocker(ctx, 10*time.Second, "network", "inspect", name)
	if err != nil {
		return nil, fmt.Errorf("docker network inspect %s: %w", name, err)
	}
	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parse network inspect: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("network %s not found", name)
	}
	return result[0], nil
}

// PruneNetworks 清理未被任何容器使用的网络（docker network prune -f 语义）。
func (r *Repository) PruneNetworks(ctx context.Context) (string, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		reports, err := network.Prune(ctx, &network.PruneOptions{})
		if err == nil {
			return fmt.Sprintf("已清理 %d 个未使用网络", len(reports)), nil
		}
		r.logger.Warn().Err(err).Msg("podman network prune failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, 30*time.Second, "network", "prune", "-f")
	return out, err
}

// NetworkCreate 创建网络（bridge 驱动 + 可选子网）。podman 优先，CLI 兜底。
func (r *Repository) NetworkCreate(ctx context.Context, name, driver, subnet string) error {
	if r.podman != nil && r.podman.IsAvailable() {
		spec := libnetworkTypes.Network{Name: name, Driver: driver}
		if subnet != "" {
			cidr, cerr := libnetworkTypes.ParseCIDR(strings.TrimSpace(subnet))
			if cerr != nil {
				return fmt.Errorf("非法子网: %s", subnet)
			}
			spec.Subnets = []libnetworkTypes.Subnet{{Subnet: cidr}}
		}
		if _, err := network.Create(ctx, &spec); err == nil {
			return nil
		} else {
			r.logger.Warn().Err(err).Msg("podman network create failed, falling back to docker CLI")
		}
	}
	args := []string{"network", "create", "--driver", driver}
	if subnet != "" {
		args = append(args, "--subnet", subnet)
	}
	args = append(args, name)
	_, err := runDocker(ctx, 30*time.Second, args...)
	return err
}
