// 镜像接口：列表（JSON）、拉取（text/plain 流式，进度终端实时呈现）、批量删除。
package handler

import (
	"fmt"
	"net/http"

	v1 "dockge/api/v1"
	"dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// ImageHandler 承载镜像列表、拉取与删除接口。
type ImageHandler struct {
	*Handler
	imageService service.ImageService
}

// NewImageHandler 构造镜像处理器，由注入容器调用。
func NewImageHandler(i do.Injector) (*ImageHandler, error) {
	return &ImageHandler{
		Handler:      do.MustInvoke[*Handler](i),
		imageService: do.MustInvoke[service.ImageService](i),
	}, nil
}

// List 处理镜像列表查询请求。
func (h *ImageHandler) List(ctx *gin.Context) {
	list, err := h.imageService.List(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, v1.ImageListData{List: list})
}

// Pull 处理手动拉取镜像请求：流式响应，与 StackHandler.Op 同构。
func (h *ImageHandler) Pull(ctx *gin.Context) {
	var req v1.ImagePullRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	w := ctx.Writer
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher := flushWriter{w: w, f: w}
	if err := h.imageService.PullStream(ctx, flusher, req.Image); err != nil {
		fmt.Fprintf(flusher, "\n[error] %s\n", err.Error())
	}
}

// Delete 处理批量删除镜像请求。
func (h *ImageHandler) Delete(ctx *gin.Context) {
	var req v1.ImageDeleteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	data, err := h.imageService.Delete(ctx, req.Images)
	if err != nil {
		if data != nil {
			v1.HandleError(ctx, http.StatusInternalServerError, err, data)
			return
		}
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}
