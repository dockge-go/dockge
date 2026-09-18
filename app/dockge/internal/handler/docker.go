package handler

import (
	"net/http"
	"sync"
	"time"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/service"
	"dockge/app/dockge/internal/version"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// DockerHandler 承载健康检查与版本检查接口（一比一复刻页面集所需的全部
// 「docker」命名空间端点；资源管理端点已随上游对齐删除）。
type DockerHandler struct {
	*Handler
	authService  service.AuthService
	versionCache versionCheckCache
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
		Handler:      do.MustInvoke[*Handler](i),
		authService:  do.MustInvoke[service.AuthService](i),
		versionCache: versionCheckCache{ttl: 5 * time.Minute},
	}, nil
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
