package handler

import (
	v1 "dockge/api/v1"
	"dockge/internal/repository"
	"dockge/internal/version"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// DockerHandler 提供免鉴权健康检查，供容器编排探测面板与运行时状态。
type DockerHandler struct {
	Repository *repository.Repository `do:""`
}

// NewDockerHandler 构造 docker 处理器，由注入容器调用。
func NewDockerHandler(i do.Injector) (*DockerHandler, error) {
	return do.MustInvokeStruct[*DockerHandler](i), nil
}

// Health 健康检查端点：始终 200（面板本身可用），body 附运行时自检结果，
// 便于容器编排与用户定位「面板能开但栈操作失败」。
func (h *DockerHandler) Health(ctx *gin.Context) {
	v1.HandleSuccess(ctx, gin.H{
		"status":  "ok",
		"version": version.Version,
		"runtime": h.Repository.Preflight(ctx.Request.Context()),
	})
}
