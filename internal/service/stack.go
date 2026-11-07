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
	"dockge/pkg/log"

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
	DeletePlan(ctx context.Context, name string) (*v1.StackDeletePlan, error)
	StreamOp(ctx context.Context, w io.Writer, name, op string) error
	ServiceOp(ctx context.Context, name, service, op string) (*v1.StackOpResponse, error)
	Stats(ctx context.Context, name string) ([]v1.ContainerStat, error)
	Networks(ctx context.Context) ([]string, error)
}

// NewStackService 构造栈服务，由注入容器调用。
func NewStackService(i do.Injector) (StackService, error) {
	return &stackService{
		repo:   do.MustInvoke[*repository.Repository](i),
		logger: do.MustInvoke[*log.Logger](i),
	}, nil
}

type stackService struct {
	repo   *repository.Repository
	logger *log.Logger
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
		return nil, fmt.Errorf("%w: 非法栈名 %q（仅允许小写字母、数字、下划线、连字符）", v1.ErrBadRequest, name)
	}
	stack, err := s.repo.Get(ctx, name)
	if err != nil {
		return nil, stackError(err)
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
		Containers: containers, URLs: urls, RuntimeError: stack.RuntimeError,
	}, nil
}

// Save 校验栈名与 YAML 后写入文件；isAdd 时创建目录并拒绝重名。
func (s *stackService) Save(ctx context.Context, req *v1.StackSaveRequest, isAdd bool) error {
	name := strings.ToLower(strings.TrimSpace(req.Name))
	if !StackNamePattern.MatchString(name) {
		return fmt.Errorf("%w: 非法栈名 %q，仅允许小写字母、数字、下划线与连字符", v1.ErrBadRequest, req.Name)
	}
	if strings.TrimSpace(req.Yaml) == "" {
		return fmt.Errorf("%w: compose 内容不能为空", v1.ErrBadRequest)
	}
	var doc any
	if err := yaml.Unmarshal([]byte(req.Yaml), &doc); err != nil {
		return fmt.Errorf("%w: YAML 语法错误: %s", v1.ErrBadRequest, err.Error())
	}
	mu, _ := opMutex.LoadOrStore(name, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	stack := &model.Stack{Name: name, Yaml: req.Yaml, Env: req.Env, ComposeFileName: "compose.yaml"}
	if err := s.repo.Save(ctx, stack, isAdd); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return fmt.Errorf("%w: 栈 %s 已存在", v1.ErrConflict, name)
		}
		return stackError(err)
	}
	return nil
}

// Delete 运行时清理和核验全部成功后才删除配置与栈内数据目录。
func (s *stackService) Delete(ctx context.Context, name string) (*v1.StackOpResponse, error) {
	if !StackNamePattern.MatchString(name) {
		return nil, fmt.Errorf("%w: 非法栈名 %q", v1.ErrBadRequest, name)
	}
	mu, _ := opMutex.LoadOrStore(name, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	output, err := s.repo.CleanStackResources(ctx, name)
	if err != nil {
		return nil, stackError(err)
	}
	if err = s.repo.Delete(ctx, name); err != nil {
		return nil, fmt.Errorf("运行时已清理，但栈目录删除失败: %w", err)
	}
	return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)}, nil
}

// DeletePlan 在确认前展示清理资源、保留引用和阻塞原因。
func (s *stackService) DeletePlan(ctx context.Context, name string) (*v1.StackDeletePlan, error) {
	if !StackNamePattern.MatchString(name) {
		return nil, fmt.Errorf("%w: 非法栈名 %q", v1.ErrBadRequest, name)
	}
	plan, err := s.repo.PlanStackDelete(ctx, name)
	return plan, stackError(err)
}

// StreamOp 流式执行栈生命周期操作：compose 输出逐行写入 w（进度终端实时流）。
// 与 Op 互斥同一把栈锁；校验失败在写出任何内容前返回错误。
func (s *stackService) StreamOp(ctx context.Context, w io.Writer, name, op string) error {
	if !StackNamePattern.MatchString(name) {
		return fmt.Errorf("%w: 非法栈名 %q（仅允许小写字母、数字、下划线、连字符）", v1.ErrBadRequest, name)
	}
	switch op {
	case "start", "stop", "restart", "down", "update":
	default:
		return fmt.Errorf("%w: 不支持的操作 %q（可选 start/stop/restart/down/update）", v1.ErrBadRequest, op)
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
		return nil, fmt.Errorf("%w: 非法栈名 %q（仅允许小写字母、数字、下划线、连字符）", v1.ErrBadRequest, name)
	}
	output, err := s.repo.ServiceOp(ctx, name, service, op)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, stackError(err)
	}
	if err != nil {
		return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)},
			fmt.Errorf("%w: %s", v1.ErrDockerError, err.Error())
	}
	return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)}, nil
}

// Stats 返回栈内容器的即时资源占用。
func (s *stackService) Stats(ctx context.Context, name string) ([]v1.ContainerStat, error) {
	if !StackNamePattern.MatchString(name) {
		return nil, fmt.Errorf("%w: 非法栈名 %q（仅允许小写字母、数字、下划线、连字符）", v1.ErrBadRequest, name)
	}
	stats, err := s.repo.StackStats(ctx, name)
	if err != nil {
		return nil, stackError(err)
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
		Managed: stack.Managed, ComposeFileName: stack.ComposeFileName, ConfigFiles: stack.ConfigFiles, RuntimeError: stack.RuntimeError,
	}
}

// stackError 保留仓储层栈名、路径与原因，同时让 HTTP 正确区分范围外栈和运行时故障。
func stackError(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("%w: %w", v1.ErrNotFound, err)
	}
	return err
}
