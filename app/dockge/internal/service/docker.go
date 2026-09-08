package service

import (
	"context"
	"fmt"
	"time"

	v1 "dockge/app/dockge/api/v1"

	"github.com/samber/do/v2"
)

// DockerService 提供宿主 docker 信息与容器总览用例。
type DockerService interface {
	Version(ctx context.Context) (*v1.DockerVersionData, error)
	Containers(ctx context.Context) (*v1.DockerContainersData, error)
	Info(ctx context.Context) (*v1.DockerInfoData, error)
	Networks(ctx context.Context) ([]string, error)
	NetworkInspect(ctx context.Context, name string) (map[string]any, error)
	Stats(ctx context.Context) (*v1.DockerStatsData, error)
	StatsStream(ctx context.Context) (<-chan *v1.DockerStatsData, error)
	StopContainer(ctx context.Context, id string) error
	StartContainer(ctx context.Context, id string) error
	RestartContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string) error
	ContainerLogsStream(ctx context.Context, id string, tail int) (<-chan string, error)
	RemoveNetwork(ctx context.Context, name string) error
	ListImages(ctx context.Context) ([]v1.DockerImageData, error)
	RemoveImage(ctx context.Context, id string) error
	ListVolumes(ctx context.Context) ([]v1.DockerVolumeData, error)
	RemoveVolume(ctx context.Context, name string) error
	PullImage(ctx context.Context, reference string) (*v1.StackOpResponse, error)
	ContainerStats(ctx context.Context) ([]v1.ContainerStatData, error)
	ContainerInspect(ctx context.Context, id string) (map[string]any, error)
	NetworkCreate(ctx context.Context, name, driver, subnet string) error
	DockerDf(ctx context.Context) ([]v1.DockerDfCategory, error)
	PruneImages(ctx context.Context) (*v1.StackOpResponse, error)
	PruneContainers(ctx context.Context) (*v1.StackOpResponse, error)
	PruneNetworks(ctx context.Context) (*v1.StackOpResponse, error)
	PruneVolumes(ctx context.Context) (*v1.StackOpResponse, error)
}

// NewDockerService 构造 docker 服务，由注入容器调用。
func NewDockerService(i do.Injector) (DockerService, error) {
	return &dockerService{Service: do.MustInvoke[*Service](i)}, nil
}

// dockerService 实现 DockerService。
type dockerService struct {
	*Service
}

// 编译期校验 dockerService 实现 DockerService 全部方法。
var _ DockerService = (*dockerService)(nil)

// Version 返回 docker 服务端版本摘要。
func (s *dockerService) Version(ctx context.Context) (*v1.DockerVersionData, error) {
	server, err := s.repo.DockerVersion(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.DockerVersionData{
		Version:    str(server["Version"]),
		APIVersion: str(server["ApiVersion"]),
		OS:         str(server["Os"]),
		Arch:       str(server["Arch"]),
	}, nil
}

// Containers 返回宿主机全部容器。
func (s *dockerService) Containers(ctx context.Context) (*v1.DockerContainersData, error) {
	containers, err := s.repo.DockerContainers(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]v1.DockerContainerData, 0, len(containers))
	for _, c := range containers {
		list = append(list, v1.DockerContainerData{
			ID: c.ID, Name: c.Name, Image: c.Image,
			State: c.State, Status: c.Status, Ports: c.Ports,
		})
	}
	return &v1.DockerContainersData{List: list}, nil
}

// Info 汇总 docker 版本、栈与容器的运行计数。
func (s *dockerService) Info(ctx context.Context) (*v1.DockerInfoData, error) {
	version, err := s.Version(ctx)
	if err != nil {
		return nil, err
	}
	info := &v1.DockerInfoData{
		Version: version.Version, OS: version.OS, Arch: version.Arch,
	}
	stacks, err := s.repo.List(ctx)
	if err == nil {
		info.StacksTotal = len(stacks)
		for _, stack := range stacks {
			if stack.Status == 3 {
				info.StacksRunning++
			}
		}
	}
	containers, err := s.repo.DockerContainers(ctx)
	if err == nil {
		info.ContainersTotal = len(containers)
		for _, c := range containers {
			if c.State == "running" {
				info.ContainersRunning++
			}
		}
	}
	return info, nil
}

// Networks 返回本机全部 docker 网络名称。
func (s *dockerService) Networks(ctx context.Context) ([]string, error) {
	return s.repo.DockerNetworks(ctx)
}

// NetworkInspect 返回指定网络的详细信息。
func (s *dockerService) NetworkInspect(ctx context.Context, name string) (map[string]any, error) {
	return s.repo.NetworkInspect(ctx, name)
}

// Stats 返回宿主机系统级 CPU/内存使用率采样 + 运行中容器的单次资源采样。
func (s *dockerService) Stats(ctx context.Context) (*v1.DockerStatsData, error) {
	cpu, memPerc, memUsedMB, memTotalMB, err := s.repo.DockerStats(ctx)
	if err != nil {
		return nil, err
	}
	data := &v1.DockerStatsData{
		CPUUsage:   cpu,
		MemUsage:   memUsedMB,
		MemPercent: memPerc,
		MemTotalMB: memTotalMB,
	}
	// 容器采样失败不影响系统统计的返回
	if ctrs, cerr := s.repo.ContainerStats(ctx); cerr == nil {
		data.Containers = make([]v1.ContainerStatData, 0, len(ctrs))
		for _, c := range ctrs {
			data.Containers = append(data.Containers, v1.ContainerStatData{
				ID: c.ID, Name: c.Name, CPU: c.CPUPercent,
				MemPercent: c.MemPercent, MemUsage: c.MemUsage,
			})
		}
	}
	return data, nil
}

// ContainerStats 返回运行中容器的资源采样。
func (s *dockerService) ContainerStats(ctx context.Context) ([]v1.ContainerStatData, error) {
	ctrs, err := s.repo.ContainerStats(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]v1.ContainerStatData, 0, len(ctrs))
	for _, c := range ctrs {
		out = append(out, v1.ContainerStatData{
			ID: c.ID, Name: c.Name, CPU: c.CPUPercent,
			MemPercent: c.MemPercent, MemUsage: c.MemUsage,
		})
	}
	return out, nil
}

// ContainerInspect 返回容器底层详情。
func (s *dockerService) ContainerInspect(ctx context.Context, id string) (map[string]any, error) {
	return s.repo.ContainerInspect(ctx, id)
}

// NetworkCreate 创建网络。
func (s *dockerService) NetworkCreate(ctx context.Context, name, driver, subnet string) error {
	if !stackNamePattern.MatchString(name) {
		return fmt.Errorf("%w: 网络名只能包含小写字母、数字、下划线与连字符", v1.ErrBadRequest)
	}
	return s.repo.NetworkCreate(ctx, name, driver, subnet)
}

// DockerDf 返回镜像/容器/卷的磁盘占用汇总。
func (s *dockerService) DockerDf(ctx context.Context) ([]v1.DockerDfCategory, error) {
	list, err := s.repo.DockerDf(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]v1.DockerDfCategory, 0, len(list))
	for _, c := range list {
		out = append(out, v1.DockerDfCategory{
			Type: c.Type, Count: c.Count, Active: c.Active,
			Size: c.Size, Reclaimable: c.Reclaimable,
		})
	}
	return out, nil
}

// StopContainer 停止指定容器（podman 优先，10 秒宽限）。

func (s *dockerService) StopContainer(ctx context.Context, id string) error {
	return s.repo.StopContainer(ctx, id)
}

// StatsStream 持续采样系统 CPU/内存并推送（每 2s 一次），随 ctx 取消结束。
func (s *dockerService) StatsStream(ctx context.Context) (<-chan *v1.DockerStatsData, error) {
	out := make(chan *v1.DockerStatsData, 4)
	go func() {
		defer close(out)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		push := func() {
			data, err := s.Stats(ctx)
			if err != nil {
				return
			}
			select {
			case <-ctx.Done():
			case out <- data:
			}
		}
		push()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				push()
			}
		}
	}()
	return out, nil
}

// ContainerLogsStream 容器日志实时流。
func (s *dockerService) ContainerLogsStream(ctx context.Context, id string, tail int) (<-chan string, error) {
	return s.repo.ContainerLogsStream(ctx, id, tail)
}

// RemoveNetwork 删除 docker 网络。
func (s *dockerService) RemoveNetwork(ctx context.Context, name string) error {
	return s.repo.RemoveNetwork(ctx, name)
}

// ListImages 返回本地镜像列表。
func (s *dockerService) ListImages(ctx context.Context) ([]v1.DockerImageData, error) {
	list, err := s.repo.DockerImages(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]v1.DockerImageData, 0, len(list))
	for _, img := range list {
		out = append(out, v1.DockerImageData{
			ID: img.ID, Repo: img.Repo, Tag: img.Tag, Size: img.Size, Created: img.Created,
		})
	}
	return out, nil
}

// RemoveImage 删除本地镜像。
func (s *dockerService) RemoveImage(ctx context.Context, id string) error {
	return s.repo.RemoveImage(ctx, id)
}

// ListVolumes 返回本地数据卷列表。
func (s *dockerService) ListVolumes(ctx context.Context) ([]v1.DockerVolumeData, error) {
	list, err := s.repo.DockerVolumes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]v1.DockerVolumeData, 0, len(list))
	for _, v := range list {
		out = append(out, v1.DockerVolumeData{Name: v.Name, Driver: v.Driver})
	}
	return out, nil
}

// RemoveVolume 删除数据卷。
func (s *dockerService) RemoveVolume(ctx context.Context, name string) error {
	return s.repo.RemoveVolume(ctx, name)
}

// PullImage 拉取镜像（耗时，10 分钟超时）。
func (s *dockerService) PullImage(ctx context.Context, reference string) (*v1.StackOpResponse, error) {
	out, err := s.repo.PullImage(ctx, reference)
	if err != nil {
		return &v1.StackOpResponse{Output: out}, fmt.Errorf("%w: %s", v1.ErrDockerError, err.Error())
	}
	return &v1.StackOpResponse{Output: out}, nil
}

// PruneImages 清理未使用镜像。
func (s *dockerService) PruneImages(ctx context.Context) (*v1.StackOpResponse, error) {
	out, err := s.repo.PruneImages(ctx)
	return &v1.StackOpResponse{Output: out}, err
}

// PruneContainers 清理已停止容器。
func (s *dockerService) PruneContainers(ctx context.Context) (*v1.StackOpResponse, error) {
	out, err := s.repo.PruneContainers(ctx)
	return &v1.StackOpResponse{Output: out}, err
}

// PruneNetworks 清理未使用网络。
func (s *dockerService) PruneNetworks(ctx context.Context) (*v1.StackOpResponse, error) {
	out, err := s.repo.PruneNetworks(ctx)
	return &v1.StackOpResponse{Output: out}, err
}

// PruneVolumes 清理未使用数据卷。
func (s *dockerService) PruneVolumes(ctx context.Context) (*v1.StackOpResponse, error) {
	out, err := s.repo.PruneVolumes(ctx)
	return &v1.StackOpResponse{Output: out}, err
}

// StartContainer 启动容器。
func (s *dockerService) StartContainer(ctx context.Context, id string) error {
	return s.repo.StartContainer(ctx, id)
}

// RestartContainer 重启容器。
func (s *dockerService) RestartContainer(ctx context.Context, id string) error {
	return s.repo.RestartContainer(ctx, id)
}

// RemoveContainer 强制删除指定容器（含运行中）。

func (s *dockerService) RemoveContainer(ctx context.Context, id string) error {
	return s.repo.RemoveContainer(ctx, id)
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
