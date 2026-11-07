package service

import (
	"context"
	v1 "dockge/api/v1"
	"dockge/internal/repository"
	"fmt"
	"github.com/samber/do/v2"
	"strconv"
)

// ResourceService 统一资源查询与动作校验，传输层不直接拼接 CLI。
type ResourceService struct{ repo *repository.Repository }

// NewResourceService 由容器注入运行时仓储。
func NewResourceService(i do.Injector) (*ResourceService, error) {
	return &ResourceService{repo: do.MustInvoke[*repository.Repository](i)}, nil
}

// Snapshot 独立收集资源与错误，客户端必须展示失败来源。
func (s *ResourceService) Snapshot(ctx context.Context) *v1.ResourceSnapshot {
	q := &v1.ResourceSnapshot{Errors: map[string]string{}, Runtime: s.repo.Preflight(ctx).CLI}
	var err error
	q.Containers, err = s.repo.ResourceContainers(ctx)
	if err != nil {
		q.Errors["containers"] = err.Error()
		q.Containers = []v1.ResourceContainer{}
	}
	q.Images, err = s.repo.ResourceImages(ctx, q.Containers)
	if err != nil {
		q.Errors["images"] = err.Error()
		q.Images = []v1.ResourceImage{}
	}
	q.Volumes, err = s.repo.ResourceVolumes(ctx, q.Containers)
	if err != nil {
		q.Errors["volumes"] = err.Error()
		q.Volumes = []v1.ResourceVolume{}
	}
	if q.Errors["containers"] != "" {
		q.Errors["images"] = "无法核对镜像引用：" + q.Errors["containers"]
		q.Errors["volumes"] = "无法核对卷引用：" + q.Errors["containers"]
	}
	if err = s.repo.ResourceVolumeUsage(ctx, q.Volumes); err != nil {
		q.Errors["volumeUsage"] = err.Error()
	}
	q.Networks, err = s.repo.ResourceNetworks(ctx)
	if err != nil {
		q.Errors["networks"] = err.Error()
		q.Networks = []v1.ResourceNetwork{}
	}
	if err = s.repo.ResourceStatistics(ctx, q.Containers); err != nil {
		q.Errors["stats"] = err.Error()
	}
	var errs map[string]string
	q.Info, q.Disk, errs = s.repo.ResourceSystem(ctx)
	for k, v := range errs {
		q.Errors[k] = v
	}
	return q
}

// Action 校验标识符与新值，不能把用户输入当作 CLI 选项。
func (s *ResourceService) Action(ctx context.Context, kind, id, op string, q v1.ResourceActionRequest) (string, error) {
	allowed := map[string]map[string]bool{"containers": {"start": true, "stop": true, "restart": true, "delete": true, "rename": true, "commit": true}, "images": {"delete": true, "tag": true}, "volumes": {"delete": true}, "networks": {"delete": true}, "buildCache": {"delete": true}}
	if !allowed[kind][op] {
		return "", fmt.Errorf("%w: 不支持资源操作 %s/%s", v1.ErrBadRequest, kind, op)
	}
	if kind == "buildCache" && id != "unused" {
		return "", fmt.Errorf("%w: 构建缓存仅支持 unused 清理，实际 id=%q", v1.ErrBadRequest, id)
	}
	if !repository.ImageRefPattern.MatchString(id) {
		return "", fmt.Errorf("%w: 非法 %s 标识符 %q", v1.ErrBadRequest, kind, id)
	}
	if op == "rename" && !repository.ContainerIDPattern.MatchString(q.Value) || (op == "tag" || op == "commit") && !repository.ImageRefPattern.MatchString(q.Value) {
		return "", fmt.Errorf("%w: 操作 %s 的 value %q 不符合名称/镜像引用格式", v1.ErrBadRequest, op, q.Value)
	}
	return s.repo.ResourceAction(ctx, kind, id, op, q.Value)
}

// Create 校验端口与必填值，失败返回实际字段和数值。
func (s *ResourceService) Create(ctx context.Context, kind string, q v1.ResourceCreateRequest) (string, error) {
	if !repository.ContainerIDPattern.MatchString(q.Name) {
		return "", fmt.Errorf("%w: name %q 必须字母数字开头，仅含字母数字、点、下划线、连字符", v1.ErrBadRequest, q.Name)
	}
	if kind == "containers" && !repository.ImageRefPattern.MatchString(q.Image) {
		return "", fmt.Errorf("%w: 非法 image %q", v1.ErrBadRequest, q.Image)
	}
	for field, value := range map[string]string{"hostPort": q.HostPort, "containerPort": q.ContainerPort} {
		if value != "" {
			n, e := strconv.Atoi(value)
			if e != nil || n < 1 || n > 65535 {
				return "", fmt.Errorf("%w: %s=%q，端口必须是 1–65535 的整数", v1.ErrBadRequest, field, value)
			}
		}
	}
	if q.HostPort != "" && q.ContainerPort == "" {
		return "", fmt.Errorf("%w: 填写 hostPort 时 containerPort 不能为空", v1.ErrBadRequest)
	}
	if q.Driver != "" && !repository.ContainerIDPattern.MatchString(q.Driver) {
		return "", fmt.Errorf("%w: 非法 driver %q", v1.ErrBadRequest, q.Driver)
	}
	return s.repo.ResourceCreate(ctx, kind, q)
}

// History 经标识符校验返回真实镜像层。
func (s *ResourceService) History(ctx context.Context, id string) ([]map[string]string, error) {
	if !repository.ImageRefPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: 非法镜像 %q", v1.ErrBadRequest, id)
	}
	return s.repo.ResourceHistory(ctx, id)
}

// System 轻量查询服务器指标，不调用容器资源 CLI，也不依赖监控服务。
func (s *ResourceService) System(ctx context.Context) *v1.HostSystem {
	return s.repo.HostSystem(ctx)
}
