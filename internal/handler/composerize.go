package handler

import (
	"net/http"

	v1 "dockge/api/v1"
	"dockge/internal/composerize"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// ComposerizeHandler 承载 composerize（docker run → compose）接口。
type ComposerizeHandler struct {
	*Handler
}

// NewComposerizeHandler 构造 composerize 处理器，由注入容器调用。
func NewComposerizeHandler(i do.Injector) (*ComposerizeHandler, error) {
	return &ComposerizeHandler{Handler: do.MustInvoke[*Handler](i)}, nil
}

// Convert 处理 docker run 命令到 compose.yaml 的转换请求。
func (h *ComposerizeHandler) Convert(ctx *gin.Context) {
	var req v1.ComposerizeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	template, err := composerize.Convert(req.DockerRunCommand)
	if err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, &v1.Error{Code: 400, Message: err.Error()}, nil)
		return
	}
	v1.HandleSuccess(ctx, v1.ComposerizeResponse{ComposeTemplate: template})
}
