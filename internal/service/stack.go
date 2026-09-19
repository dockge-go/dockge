package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	v1 "dockge/api/v1"
	"dockge/internal/model"
	"dockge/internal/repository"

	"github.com/samber/do/v2"
	"gopkg.in/yaml.v3"
)

var StackNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

var opMutex sync.Map

type StackService interface {
	List(ctx context.Context, filter string) (*v1.StackListData, error)
	Get(ctx context.Context, name string) (*v1.StackDetailData, error)
	Save(ctx context.Context, req *v1.StackSaveRequest, isAdd bool) error
	Delete(ctx context.Context, name string) (*v1.StackOpResponse, error)
	StreamOp(ctx context.Context, w io.Writer, name, op string) error
	ServiceOp(ctx context.Context, name, service, op string) (*v1.StackOpResponse, error)
	Stats(ctx context.Context, name string) ([]v1.ContainerStat, error)
	Networks(ctx context.Context) ([]string, error)
}

// NewStackService 构造栈服务，由注入容器调用。
func NewStackService(i do.Injector) (StackService, error) {
	return &stackService{Service: do.MustInvoke[*Service](i)}, nil
}

type stackService struct {
	*Service
}

// List 返回栈列表（filter 非空时按名称子串过滤，忽略大小写）。
func (s *stackService) List(ctx context.Context, filter string) (*v1.StackListData, error) {
	stacks, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	data := &v1.StackListData{List: make([]v1.StackSummaryData, 0, len(stacks))}
	for _, stack := range stacks {
		if filter != "" && !strings.Contains(strings.ToLower(stack.Name), strings.ToLower(filter)) {
			continue
		}
		data.List = append(data.List, stackSummary(stack))
	}
	return data, nil
}

// Get 返回栈详情：文件内容、状态、容器列表与 x-dockge.urls。
func (s *stackService) Get(ctx context.Context, name string) (*v1.StackDetailData, error) {
	if !StackNamePattern.MatchString(name) {
		return nil, v1.ErrBadRequest
	}
	stack, err := s.repo.Get(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, err
	}
	containers := make([]v1.StackContainer, 0, len(stack.Containers))
	for _, c := range stack.Containers {
		ports := make([]v1.PortMapping, 0, len(c.Ports))
		for _, p := range c.Ports {
			ports = append(ports, v1.PortMapping{
				HostIP: p.HostIP, HostPort: p.HostPort,
				ContainerPort: p.ContainerPort, Protocol: p.Protocol,
			})
		}
		containers = append(containers, v1.StackContainer{
			ID: c.ID, Name: c.Name, Service: c.Service,
			Image: c.Image, State: c.State, Status: c.Status, Ports: ports,
		})
	}
	urls := repository.ParseXDockgeURLs(stack.Yaml, stack.Env)
	return &v1.StackDetailData{
		Name: stack.Name, Status: int(stack.Status),
		Managed: stack.Managed, ComposeFileName: stack.ComposeFileName,
		ConfigFiles: stack.ConfigFiles, Yaml: stack.Yaml, Env: stack.Env,
		Containers: containers, URLs: urls,
	}, nil
}

// Save 校验栈名与 YAML 后写入文件；isAdd 时创建目录并拒绝重名。
func (s *stackService) Save(ctx context.Context, req *v1.StackSaveRequest, isAdd bool) error {
	name := strings.ToLower(strings.TrimSpace(req.Name))
	if !StackNamePattern.MatchString(name) {
		return fmt.Errorf("%w: 栈名只能包含小写字母、数字、下划线与连字符", v1.ErrBadRequest)
	}
	if strings.TrimSpace(req.Yaml) == "" {
		return fmt.Errorf("%w: compose 内容不能为空", v1.ErrBadRequest)
	}
	var doc any
	if err := yaml.Unmarshal([]byte(req.Yaml), &doc); err != nil {
		return fmt.Errorf("%w: YAML 语法错误: %s", v1.ErrBadRequest, err.Error())
	}
	stack := &model.Stack{Name: name, Yaml: req.Yaml, Env: req.Env, ComposeFileName: "compose.yaml"}
	if err := s.repo.Save(ctx, stack, isAdd); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return fmt.Errorf("%w: 栈 %s 已存在", v1.ErrConflict, name)
		}
		if errors.Is(err, repository.ErrNotFound) {
			return v1.ErrNotFound
		}
		return err
	}
	return nil
}

// Delete 先执行 compose down 再删除栈目录；删除后仍注册于 compose 则报错。
func (s *stackService) Delete(ctx context.Context, name string) (*v1.StackOpResponse, error) {
	if !StackNamePattern.MatchString(name) {
		return nil, v1.ErrBadRequest
	}
	mu, _ := opMutex.LoadOrStore(name, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	output, err := s.repo.StackOp(ctx, name, "down")
	if err != nil {
		s.logger.WithContext(ctx).Warn().Err(err).Str("stack", name).Msg("stack down before delete")
	}
	if err := s.repo.Delete(ctx, name); err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
		output += "\n（外部栈：无本地目录可删）"
	}
	if items, lsErr := s.repo.ComposeLs(ctx); lsErr == nil {
		for _, item := range items {
			if item.Name == name {
				return nil, fmt.Errorf("%w: 容器/网络未能完全移除，项目仍注册于 docker compose：%s",
					v1.ErrDockerError, repository.TrimStackOutput(output))
			}
		}
	}
	return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)}, nil
}

// StreamOp 流式执行栈生命周期操作：compose 输出逐行写入 w（进度终端实时流）。
// 与 Op 互斥同一把栈锁；校验失败在写出任何内容前返回错误。
func (s *stackService) StreamOp(ctx context.Context, w io.Writer, name, op string) error {
	if !StackNamePattern.MatchString(name) {
		return v1.ErrBadRequest
	}
	switch op {
	case "start", "stop", "restart", "down", "update":
	default:
		return v1.ErrBadRequest
	}
	mu, _ := opMutex.LoadOrStore(name, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	return s.repo.StackOpStream(ctx, name, op, w)
}

// Networks 返回本机网络名列表（编辑器建议）。
func (s *stackService) Networks(ctx context.Context) ([]string, error) {
	return s.repo.DockerNetworkNames(ctx)
}

// ServiceOp 执行单服务生命周期操作；失败时带回 compose 输出。
func (s *stackService) ServiceOp(ctx context.Context, name, service, op string) (*v1.StackOpResponse, error) {
	if !StackNamePattern.MatchString(name) {
		return nil, v1.ErrBadRequest
	}
	output, err := s.repo.ServiceOp(ctx, name, service, op)
	if err != nil {
		return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)},
			fmt.Errorf("%w: %s", v1.ErrDockerError, err.Error())
	}
	return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)}, nil
}

// Stats 返回栈内容器的即时资源占用。
func (s *stackService) Stats(ctx context.Context, name string) ([]v1.ContainerStat, error) {
	if !StackNamePattern.MatchString(name) {
		return nil, v1.ErrBadRequest
	}
	stats, err := s.repo.StackStats(ctx, name)
	if err != nil {
		return nil, err
	}
	result := make([]v1.ContainerStat, 0, len(stats))
	for _, stat := range stats {
		result = append(result, v1.ContainerStat{
			Name: stat.Name, CPUPerc: stat.CPUPerc, MemUsage: stat.MemUsage,
			MemPerc: stat.MemPerc, NetIO: stat.NetIO, BlockIO: stat.BlockIO,
		})
	}
	return result, nil
}

func stackSummary(stack model.Stack) v1.StackSummaryData {
	return v1.StackSummaryData{
		Name: stack.Name, Status: int(stack.Status),
		Managed: stack.Managed, ComposeFileName: stack.ComposeFileName, ConfigFiles: stack.ConfigFiles,
	}
}
