// 容器域：容器总览（docker/podman ps -a）、生命周期操作（启停/重启/删除）、
// 日志实时流与容器内执行。podman client 优先，不可用时降级 docker CLI。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"go.podman.io/podman/v6/pkg/bindings/containers"
	podmanTypes "go.podman.io/podman/v6/pkg/domain/entities/types"

	"dockge/app/dockge/internal/model"
)

type dockerPsItem struct {
	ID      string `json:"ID"`
	Names   string `json:"Names"`
	Image   string `json:"Image"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Ports   string `json:"Ports"`
	Network string `json:"Network"`
	Labels  string `json:"Labels"`
}

// stackFromLabels 从容器 labels 提取 compose 项目名。
func stackFromLabels(labels string) string {
	for _, kv := range strings.Split(labels, ",") {
		if strings.HasPrefix(kv, "com.docker.compose.project=") {
			return strings.TrimPrefix(kv, "com.docker.compose.project=")
		}
	}
	return ""
}

// parseDockerPorts 解析 docker ps 的 Ports 列：
// "0.0.0.0:8080->80/tcp, :::8080->80/tcp, 53/udp"。无法解析的段跳过。
func parseDockerPorts(s string) []model.PortMapping {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	mappings := make([]model.PortMapping, 0)
	for _, seg := range strings.Split(s, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		m, ok := parsePortSegment(seg)
		if ok {
			mappings = append(mappings, m)
		}
	}
	return mappings
}

// parsePortSegment 解析单段端口映射，返回 (mapping, 是否可解析)。
func parsePortSegment(seg string) (model.PortMapping, bool) {
	var m model.PortMapping
	host, container, published := strings.Cut(seg, "->")
	if published {
		// host: "0.0.0.0:8080" / "[::]:8080" / "8080" / "8080-8089"
		hostIP, port := splitHostPort(host)
		hostPort, err := strconv.Atoi(firstToken(port, "-"))
		if err != nil {
			return m, false
		}
		m.HostIP, m.HostPort = hostIP, hostPort
	}
	containerPort, proto, _ := strings.Cut(container, "/")
	if proto == "" {
		proto = "tcp"
	}
	cp, err := strconv.Atoi(containerPort)
	if err != nil {
		return m, false
	}
	m.ContainerPort, m.Protocol = cp, proto
	return m, true
}

// splitHostPort 拆分 "0.0.0.0:8080"、"[::]:8080"、"8080" 为 (IP, port)。
func splitHostPort(s string) (string, string) {
	if strings.HasPrefix(s, "[") {
		if ip, port, found := strings.Cut(strings.TrimPrefix(s, "["), "]:"); found {
			return ip, port
		}
		return s, ""
	}
	if ip, port, found := strings.Cut(s, ":"); found {
		return ip, port
	}
	return "", s
}

// firstToken 取 "-" 分隔的第一段（端口范围取起点）。
func firstToken(s, sep string) string {
	if v, _, found := strings.Cut(s, sep); found {
		return v
	}
	return s
}

// DockerContainers 返回本机全部容器（docker/podman ps -a，供容器总览页）。
// 优先使用 podman Go client，不可用时降级到 docker CLI。
func (r *Repository) DockerContainers(ctx context.Context) ([]model.Container, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		items, err := containers.List(ctx, &containers.ListOptions{All: boolPtr(true)})
		if err == nil {
			result := podmanToContainers(items)
			for i := range result {
				if result[i].Stack == "" {
					result[i].Stack = items[i].Labels["com.docker.compose.project"]
				}
			}
			return result, nil
		}
		r.logger.Warn().Err(err).Msg("podman containers list failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, composeOpTimeout, "ps", "--all", "--format", "{{json .}}")
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	containers := make([]model.Container, 0)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item dockerPsItem
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse docker ps output: %w", err)
		}
		containers = append(containers, model.Container{
			ID:     item.ID,
			Name:   strings.TrimPrefix(item.Names, "/"),
			Image:  item.Image,
			State:  item.State,
			Status: item.Status,
			Ports:  parseDockerPorts(item.Ports),
			Stack:  stackFromLabels(item.Labels),
		})
	}
	return containers, nil
}

// StopContainer 停止指定容器（docker/podman stop）。
// 优先使用 podman Go client，不可用时降级到 docker CLI。
func (r *Repository) StopContainer(ctx context.Context, id string) error {
	if r.podman != nil && r.podman.IsAvailable() {
		err := containers.Stop(ctx, id, &containers.StopOptions{Timeout: uintPtr(10)})
		if err == nil {
			return nil
		}
		r.logger.Warn().Err(err).Msg("podman stop failed, falling back to docker CLI")
	}
	_, cliErr := runDocker(ctx, composeOpTimeout, "stop", id)
	return cliErr
}

// StartContainer 启动指定容器。podman client 优先，不可用时降级 docker CLI。
func (r *Repository) StartContainer(ctx context.Context, id string) error {
	if r.podman != nil && r.podman.IsAvailable() {
		err := containers.Start(ctx, id, &containers.StartOptions{})
		if err == nil {
			return nil
		}
		r.logger.Warn().Err(err).Msg("podman start failed, falling back to docker CLI")
	}
	_, err := runDocker(ctx, composeOpTimeout, "start", id)
	return err
}

// RestartContainer 重启指定容器。podman client 优先，不可用时降级 docker CLI。
func (r *Repository) RestartContainer(ctx context.Context, id string) error {
	if r.podman != nil && r.podman.IsAvailable() {
		err := containers.Restart(ctx, id, &containers.RestartOptions{})
		if err == nil {
			return nil
		}
		r.logger.Warn().Err(err).Msg("podman restart failed, falling back to docker CLI")
	}
	_, err := runDocker(ctx, composeOpTimeout, "restart", id)
	return err
}

// RemoveContainer 删除指定容器（docker/podman rm -f）。
// 优先使用 podman Go client，不可用时降级到 docker CLI。
func (r *Repository) RemoveContainer(ctx context.Context, id string) error {
	if r.podman != nil && r.podman.IsAvailable() {
		_, err := containers.Remove(ctx, id, &containers.RemoveOptions{Force: boolPtr(true)})
		if err == nil {
			return nil
		}
		r.logger.Warn().Err(err).Msg("podman remove failed, falling back to docker CLI")
	}
	_, cliErr := runDocker(ctx, composeOpTimeout, "rm", "-f", id)
	return cliErr
}

// ContainerLogsStream 返回指定容器日志的实时流。podman client 优先（双通道扇入），
// 不可用时降级 docker CLI（docker logs -f）。
// 返回读取端，调用方负责在 goroutine 中读取直到 ctx 取消。
func (r *Repository) ContainerLogsStream(ctx context.Context, id string, tail int) (<-chan string, error) {
	if tail <= 0 || tail > 5000 {
		tail = 200
	}
	if !ContainerIDPattern.MatchString(id) {
		return nil, fmt.Errorf("非法容器 ID: %s", id)
	}
	if r.podman != nil && r.podman.IsAvailable() {
		ch := make(chan string, 64)
		stdout := make(chan string)
		stderr := make(chan string)
		go func() {
			defer close(ch)
			if err := containers.Logs(ctx, id, &containers.LogOptions{
				Follow:     boolPtr(true),
				Tail:       strPtr(strconv.Itoa(tail)),
				Stdout:     boolPtr(true),
				Stderr:     boolPtr(true),
				Timestamps: boolPtr(true),
			}, stdout, stderr); err != nil {
				ch <- "podman logs: " + err.Error() + "\n"
			}
		}()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case line, ok := <-stdout:
					if !ok {
						stdout = nil
						if stderr == nil {
							return
						}
						continue
					}
					select {
					case <-ctx.Done():
						return
					case ch <- line + "\n":
					}
				case line, ok := <-stderr:
					if !ok {
						stderr = nil
						if stdout == nil {
							return
						}
						continue
					}
					select {
					case <-ctx.Done():
						return
					case ch <- line + "\n":
					}
				}
			}
		}()
		return ch, nil
	}
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", strconv.Itoa(tail), "-f", id)
	return streamCmd(ctx, cmd)
}

func podmanToContainers(items []podmanTypes.ListContainer) []model.Container {
	result := make([]model.Container, 0, len(items))
	for _, ic := range items {
		name := ""
		if len(ic.Names) > 0 {
			name = ic.Names[0]
		}
		state := strings.ToLower(ic.State)
		status := ic.Status
		if status == "" {
			status = state
		}
		ports := make([]model.PortMapping, 0, len(ic.Ports))
		for _, p := range ic.Ports {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			ports = append(ports, model.PortMapping{
				HostIP:        p.HostIP,
				HostPort:      int(p.HostPort),
				ContainerPort: int(p.ContainerPort),
				Protocol:      proto,
			})
		}
		result = append(result, model.Container{
			ID:     shortID(ic.ID),
			Name:   name,
			Image:  ic.Image,
			State:  state,
			Status: status,
			Ports:  ports,
		})
	}
	return result
}

// ContainerInspect 返回容器底层详情（env/mounts/网络等完整信息）。
// podman bindings 优先；不可用时降级 docker inspect CLI。
func (r *Repository) ContainerInspect(ctx context.Context, id string) (map[string]any, error) {
	if !ContainerIDPattern.MatchString(id) {
		return nil, fmt.Errorf("非法容器 ID: %s", id)
	}
	if r.podman != nil && r.podman.IsAvailable() {
		if info, err := containers.Inspect(ctx, id, &containers.InspectOptions{}); err == nil {
			// 统一转成 map 输出（podman 与 docker 的 inspect 结构字段名有差异）
			buf, merr := json.Marshal(info)
			if merr != nil {
				return nil, merr
			}
			var asMap map[string]any
			if err := json.Unmarshal(buf, &asMap); err != nil {
				return nil, err
			}
			return asMap, nil
		} else {
			r.logger.Warn().Err(err).Msg("podman inspect failed, falling back to docker CLI")
		}
	}
	out, err := runDocker(ctx, 30*time.Second, "inspect", id)
	if err != nil {
		return nil, fmt.Errorf("docker inspect: %w", err)
	}
	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parse docker inspect output: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("container %s not found", id)
	}
	return result[0], nil
}

// PruneContainers 清理全部已停止的容器（docker container prune -f 语义）。
func (r *Repository) PruneContainers(ctx context.Context) (string, error) {
	if r.podman != nil && r.podman.IsAvailable() {
		reports, err := containers.Prune(ctx, &containers.PruneOptions{})
		if err == nil {
			return fmt.Sprintf("已清理 %d 个已停止容器", len(reports)), nil
		}
		r.logger.Warn().Err(err).Msg("podman container prune failed, falling back to docker CLI")
	}
	out, err := runDocker(ctx, composeOpTimeout, "container", "prune", "-f")
	return out, err
}
