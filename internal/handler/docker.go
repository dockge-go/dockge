package handler

import (
	v1 "dockge/api/v1"
	"dockge/internal/repository"
	"dockge/internal/version"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// DockerHandler 承载健康检查接口（一比一复刻页面集所需的全部
// 「docker」命名空间端点；资源管理端点已随上游对齐删除）。
type DockerHandler struct {
	*Handler
	repo *repository.Repository
}

// NewDockerHandler 构造 docker 处理器，由注入容器调用。
func NewDockerHandler(i do.Injector) (*DockerHandler, error) {
	return &DockerHandler{
		Handler: do.MustInvoke[*Handler](i),
		repo:    do.MustInvoke[*repository.Repository](i),
	}, nil
}

// Health 健康检查端点：始终 200（面板本身可用），body 附运行时自检结果，
// 便于容器编排与用户定位「面板能开但栈操作失败」。
func (h *DockerHandler) Health(ctx *gin.Context) {
	v1.HandleSuccess(ctx, gin.H{
		"status":  "ok",
		"version": version.Version,
		"runtime": h.repo.Preflight(ctx.Request.Context()),
	})
}
