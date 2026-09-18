package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/model"
	"dockge/app/dockge/internal/repository"

	"github.com/samber/do/v2"
	"gopkg.in/yaml.v3"
)

var stackNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// lineNumberPattern 提取报错文本中的行号；yaml.v3 与 docker compose 的
// YAML 类错误都形如 "line N: ..."。
var lineNumberPattern = regexp.MustCompile(`line (\d+)`)

// validatingPrefixPattern 剥离 compose 报错的临时文件路径前缀，避免噪音。
var validatingPrefixPattern = regexp.MustCompile(`^validating \S+: `)

var opMutex sync.Map

type StackService interface {
	List(ctx context.Context, filter string) (*v1.StackListData, error)
	Get(ctx context.Context, name string) (*v1.StackDetailData, error)
	Save(ctx context.Context, req *v1.StackSaveRequest, isAdd bool) error
	Validate(ctx context.Context, req *v1.StackValidateRequest) *v1.StackValidateResponse
	Delete(ctx context.Context, name string) (*v1.StackOpResponse, error)
	Op(ctx context.Context, name, op string) (*v1.StackOpResponse, error)
	ServiceOp(ctx context.Context, name, service, op string) (*v1.StackOpResponse, error)
	Stats(ctx context.Context, name string) ([]v1.ContainerStat, error)
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
	if !stackNamePattern.MatchString(name) {
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
		Name: stack.Name, Status: int(stack.Status), StatusLabel: stack.Status.Label(),
		Managed: stack.Managed, ComposeFileName: stack.ComposeFileName,
		ConfigFiles: stack.ConfigFiles, Yaml: stack.Yaml, Env: stack.Env,
		Containers: containers, URLs: urls,
	}, nil
}

// Save 校验栈名与 YAML 后写入文件；isAdd 时创建目录并拒绝重名。
func (s *stackService) Save(ctx context.Context, req *v1.StackSaveRequest, isAdd bool) error {
	name := strings.ToLower(strings.TrimSpace(req.Name))
	if !stackNamePattern.MatchString(name) {
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

// Validate 对草稿做语法与 compose 语义校验；结果恒为数据（不返回错误），
// 校验失败是正常响应而非请求失败。语义校验经 docker compose config 于
// 临时目录完成，不落盘、不创建资源。
func (s *stackService) Validate(ctx context.Context, req *v1.StackValidateRequest) *v1.StackValidateResponse {
	resp := &v1.StackValidateResponse{Errors: []v1.StackValidateError{}}
	var doc any
	if err := yaml.Unmarshal([]byte(req.Yaml), &doc); err != nil {
		resp.Errors = append(resp.Errors, v1.StackValidateError{Line: yamlErrorLine(err), Message: err.Error()})
		return resp
	}
	out, err := s.repo.ValidateCompose(ctx, req.Yaml, req.Env)
	if err != nil {
		resp.Errors = append(resp.Errors, parseComposeErrors(out)...)
		if len(resp.Errors) == 0 {
			// config 失败但无输出（如 docker 不可用/超时），保留原始错误供前端呈现
			resp.Errors = append(resp.Errors, v1.StackValidateError{Message: err.Error()})
		}
		return resp
	}
	resp.Valid = true
	return resp
}

// yamlErrorLine 从 yaml.v3 错误消息提取行号；无定位信息返回 0。
// v3 未导出 SyntaxError 类型，行号只存在于 "yaml: line N: ..." 消息文本中。
func yamlErrorLine(err error) int {
	if err == nil {
		return 0
	}
	if m := lineNumberPattern.FindStringSubmatch(err.Error()); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// parseComposeErrors 把 docker compose config 的多行输出拆为结构化诊断；
// 无行号的行 Line=0，由前端降级为面板列表项。
func parseComposeErrors(out string) []v1.StackValidateError {
	items := make([]v1.StackValidateError, 0)
	for _, line := range strings.Split(out, "\n") {
		msg := validatingPrefixPattern.ReplaceAllString(strings.TrimSpace(line), "")
		if msg == "" {
			continue
		}
		n := 0
		if m := lineNumberPattern.FindStringSubmatch(msg); m != nil {
			n, _ = strconv.Atoi(m[1])
		}
		items = append(items, v1.StackValidateError{Line: n, Message: msg})
	}
	return items
}

// Delete 先执行 compose down 再删除栈目录；删除后仍注册于 compose 则报错。
func (s *stackService) Delete(ctx context.Context, name string) (*v1.StackOpResponse, error) {
	if !stackNamePattern.MatchString(name) {
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

// Op 执行栈生命周期操作；失败时带回 compose 输出供前端展示。
func (s *stackService) Op(ctx context.Context, name, op string) (*v1.StackOpResponse, error) {
	if !stackNamePattern.MatchString(name) {
		return nil, v1.ErrBadRequest
	}
	switch op {
	case "start", "stop", "restart", "down", "update":
	default:
		return nil, v1.ErrBadRequest
	}
	mu, _ := opMutex.LoadOrStore(name, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	output, err := s.repo.StackOp(ctx, name, op)
	if err != nil {
		return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)},
			fmt.Errorf("%w: %s", v1.ErrDockerError, err.Error())
	}
	return &v1.StackOpResponse{Output: repository.TrimStackOutput(output)}, nil
}

// ServiceOp 执行单服务生命周期操作；失败时带回 compose 输出。
func (s *stackService) ServiceOp(ctx context.Context, name, service, op string) (*v1.StackOpResponse, error) {
	if !stackNamePattern.MatchString(name) {
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
	if !stackNamePattern.MatchString(name) {
		return nil, v1.ErrBadRequest
	}
	stats, err := s.repo.StackStats(ctx, name)
	if err != nil {
		return nil, err
	}
	result := make([]v1.ContainerStat, 0, len(stats))
	for _, stat := range stats {
		result = append(result, v1.ContainerStat{Name: stat.Name, CPUPerc: stat.CPUPerc, MemUsage: stat.MemUsage})
	}
	return result, nil
}

func stackSummary(stack model.Stack) v1.StackSummaryData {
	return v1.StackSummaryData{
		Name: stack.Name, Status: int(stack.Status), StatusLabel: stack.Status.Label(),
		Managed: stack.Managed, ComposeFileName: stack.ComposeFileName, ConfigFiles: stack.ConfigFiles,
	}
}
