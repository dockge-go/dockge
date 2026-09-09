package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/push"
	"dockge/app/dockge/internal/service"
	"dockge/app/dockge/internal/version"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// DockerHandler 承载宿主 docker 信息与容器总览接口。
type DockerHandler struct {
	*Handler
	dockerService service.DockerService
	authService   service.AuthService
	statusHub     *push.StatusHub
	versionCache  versionCheckCache
}

type versionCheckCache struct {
	mu        sync.Mutex
	data      *v1.VersionCheckResponse
	expiresAt time.Time
	ttl       time.Duration
}

func (c *versionCheckCache) get() (*v1.VersionCheckResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.expiresAt) < 0 && c.data != nil {
		return c.data, true
	}
	return nil, false
}

func (c *versionCheckCache) set(v *v1.VersionCheckResponse) {
	c.mu.Lock()
	c.data = v
	c.expiresAt = time.Now().Add(c.ttl)
	c.mu.Unlock()
}

// NewDockerHandler 构造 docker 处理器，由注入容器调用。
func NewDockerHandler(i do.Injector) (*DockerHandler, error) {
	return &DockerHandler{
		Handler:       do.MustInvoke[*Handler](i),
		dockerService: do.MustInvoke[service.DockerService](i),
		authService:   do.MustInvoke[service.AuthService](i),
		statusHub:     do.MustInvoke[*push.StatusHub](i),
		versionCache:  versionCheckCache{ttl: 5 * time.Minute},
	}, nil
}

// Version 返回 docker 服务端版本摘要。
func (h *DockerHandler) Version(ctx *gin.Context) {
	data, err := h.dockerService.Version(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Containers 返回容器总览。
func (h *DockerHandler) Containers(ctx *gin.Context) {
	data, err := h.dockerService.Containers(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// ContainerStatusStream 推送容器实时状态，不承担资源列表传输。
func (h *DockerHandler) ContainerStatusStream(ctx *gin.Context) {
	messages, last := h.statusHub.Subscribe()
	defer h.statusHub.Unsubscribe(messages)

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")
	ctx.Writer.WriteHeader(http.StatusOK)

	write := func(payload []byte) bool {
		if _, err := ctx.Writer.WriteString("data: " + string(payload) + "\n\n"); err != nil {
			return false
		}
		ctx.Writer.Flush()
		return true
	}
	if len(last) > 0 && !write(last) {
		return
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case payload := <-messages:
			if !write(payload) {
				return
			}
		case <-heartbeat.C:
			if !write(nil) {
				return
			}
		}
	}
}

// Info 返回仪表盘汇总（版本 + 栈/容器计数）。
func (h *DockerHandler) Info(ctx *gin.Context) {
	data, err := h.dockerService.Info(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Networks 返回本机全部 docker 网络名称。
func (h *DockerHandler) Networks(ctx *gin.Context) {
	list, err := h.dockerService.Networks(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"list": list})
}

// StatsStream 系统 CPU/内存的 SSE 实时推送（每 2s 一帧），替代前端轮询。
// EventSource 无法自定义请求头，认证依赖 StrictAuth 的 ?token= 支持。
func (h *DockerHandler) StatsStream(ctx *gin.Context) {
	stream, err := h.dockerService.StatsStream(ctx.Request.Context())
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")
	ctx.Writer.WriteHeader(http.StatusOK)
	done := ctx.Request.Context().Done()
	for {
		select {
		case <-done:
			return
		case data, ok := <-stream:
			if !ok {
				return
			}
			payload, err := json.Marshal(data)
			if err != nil {
				return
			}
			if _, err := ctx.Writer.WriteString("data: " + string(payload) + "\n\n"); err != nil {
				return
			}
			ctx.Writer.Flush()
		}
	}
}

// ContainerLogs 指定容器日志的 SSE 实时流（docker logs -f --tail=N）。
func (h *DockerHandler) ContainerLogs(ctx *gin.Context) {
	id := ctx.Param("id")
	tail, _ := strconv.Atoi(ctx.DefaultQuery("tail", "200"))
	stream, err := h.dockerService.ContainerLogsStream(ctx.Request.Context(), id, tail)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")
	ctx.Writer.WriteHeader(http.StatusOK)
	done := ctx.Request.Context().Done()
	for {
		select {
		case <-done:
			return
		case line, ok := <-stream:
			if !ok {
				return
			}
			if _, err := ctx.Writer.WriteString("data: " + line + "\n\n"); err != nil {
				return
			}
			ctx.Writer.Flush()
		}
	}
}

// StartContainer 启动指定容器。
func (h *DockerHandler) StartContainer(ctx *gin.Context) {
	if err := h.dockerService.StartContainer(ctx, ctx.Param("id")); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// RestartContainer 重启指定容器。
func (h *DockerHandler) RestartContainer(ctx *gin.Context) {
	if err := h.dockerService.RestartContainer(ctx, ctx.Param("id")); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// RemoveNetwork 删除指定 docker 网络。
func (h *DockerHandler) RemoveNetwork(ctx *gin.Context) {
	if err := h.dockerService.RemoveNetwork(ctx, ctx.Param("name")); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// ContainerInspect 返回容器底层详情。
func (h *DockerHandler) ContainerInspect(ctx *gin.Context) {
	data, err := h.dockerService.ContainerInspect(ctx, ctx.Param("id"))
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// NetworkCreate 创建网络。
func (h *DockerHandler) NetworkCreate(ctx *gin.Context) {
	var req v1.NetworkCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	driver := req.Driver
	if driver == "" {
		driver = "bridge"
	}
	if err := h.dockerService.NetworkCreate(ctx, strings.TrimSpace(req.Name), driver, strings.TrimSpace(req.Subnet)); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// DockerDf 返回镜像/容器/卷的磁盘占用汇总。
func (h *DockerHandler) DockerDf(ctx *gin.Context) {
	data, err := h.dockerService.DockerDf(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"list": data})
}

// DockerImages 返回本地镜像列表。
func (h *DockerHandler) DockerImages(ctx *gin.Context) {
	data, err := h.dockerService.ListImages(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"list": data})
}

// RemoveImage 删除本地镜像。
func (h *DockerHandler) RemoveImage(ctx *gin.Context) {
	if err := h.dockerService.RemoveImage(ctx, ctx.Param("id")); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// DockerVolumes 返回本地数据卷列表。
func (h *DockerHandler) DockerVolumes(ctx *gin.Context) {
	data, err := h.dockerService.ListVolumes(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"list": data})
}

// RemoveVolume 删除数据卷。
func (h *DockerHandler) RemoveVolume(ctx *gin.Context) {
	if err := h.dockerService.RemoveVolume(ctx, ctx.Param("name")); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// PruneImages 清理未使用镜像。
func (h *DockerHandler) PruneImages(ctx *gin.Context) {
	data, err := h.dockerService.PruneImages(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// PruneContainers 清理已停止容器。
func (h *DockerHandler) PruneContainers(ctx *gin.Context) {
	data, err := h.dockerService.PruneContainers(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// PruneNetworks 清理未使用网络。
func (h *DockerHandler) PruneNetworks(ctx *gin.Context) {
	data, err := h.dockerService.PruneNetworks(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// PruneVolumes 清理未使用数据卷。
func (h *DockerHandler) PruneVolumes(ctx *gin.Context) {
	data, err := h.dockerService.PruneVolumes(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// PullImage 拉取镜像（耗时操作）。
func (h *DockerHandler) PullImage(ctx *gin.Context) {
	var req v1.PullImageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Reference) == "" {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	data, err := h.dockerService.PullImage(ctx, strings.TrimSpace(req.Reference))
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Health 健康检查端点。
func (h *DockerHandler) Health(ctx *gin.Context) {
	v1.HandleSuccess(ctx, gin.H{"status": "ok", "version": version.Version})
}

// VersionCheck 检查最新版本（5分钟缓存，避免频繁请求 GitHub API）。
func (h *DockerHandler) VersionCheck(ctx *gin.Context) {
	if cached, ok := h.versionCache.get(); ok {
		v1.HandleSuccess(ctx, cached)
		return
	}
	latest, err := h.authService.GetLatestVersion(ctx)
	if err != nil {
		v1.HandleError(ctx, http.StatusInternalServerError, err, nil)
		return
	}
	v := &v1.VersionCheckResponse{CurrentVersion: version.Version}
	if latest != "" {
		v.LatestVersion = latest
		v.HasUpdate = latest != version.Version
	}
	h.versionCache.set(v)
	v1.HandleSuccess(ctx, v)
}

// StopContainer 停止指定容器。
func (h *DockerHandler) StopContainer(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := h.dockerService.StopContainer(ctx, id); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// RemoveContainer 删除指定容器。
func (h *DockerHandler) RemoveContainer(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := h.dockerService.RemoveContainer(ctx, id); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// NetworkInspect 返回网络详细信息。
func (h *DockerHandler) NetworkInspect(ctx *gin.Context) {
	name := ctx.Param("name")
	data, err := h.dockerService.NetworkInspect(ctx, name)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}
