package handler

import (
	"net/http"
	"strconv"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// StackHandler 承载 compose 栈的查询、编辑、生命周期与日志接口。
type StackHandler struct {
	*Handler
	stackService service.StackService
}

// NewStackHandler 构造栈处理器，由注入容器调用。
func NewStackHandler(i do.Injector) (*StackHandler, error) {
	return &StackHandler{
		Handler:      do.MustInvoke[*Handler](i),
		stackService: do.MustInvoke[service.StackService](i),
	}, nil
}

// List 处理栈列表查询请求。
func (h *StackHandler) List(ctx *gin.Context) {
	filter := ctx.Query("filter")
	data, err := h.stackService.List(ctx, filter)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Get 处理栈详情查询请求。
func (h *StackHandler) Get(ctx *gin.Context) {
	data, err := h.stackService.Get(ctx, ctx.Param("name"))
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Create 处理创建栈请求。
func (h *StackHandler) Create(ctx *gin.Context) {
	var req v1.StackSaveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.stackService.Save(ctx, &req, true); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// Update 处理保存栈文件请求。
func (h *StackHandler) Update(ctx *gin.Context) {
	var req v1.StackSaveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	req.Name = ctx.Param("name")
	if err := h.stackService.Save(ctx, &req, false); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// Delete 处理删除栈请求（down + 删目录）。
func (h *StackHandler) Delete(ctx *gin.Context) {
	data, err := h.stackService.Delete(ctx, ctx.Param("name"))
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Logs 处理栈组合日志的 SSE 实时流请求（docker compose logs -f --tail=N）。
// EventSource 无法自定义请求头，认证依赖 StrictAuth 的 ?token= 支持。
func (h *StackHandler) Logs(ctx *gin.Context) {
	tail, _ := strconv.Atoi(ctx.DefaultQuery("tail", "200"))
	stream, err := h.stackService.LogsStream(ctx.Request.Context(), ctx.Param("name"), tail)
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

// Op 处理栈生命周期操作（start/stop/restart/down/update）。
func (h *StackHandler) Op(ctx *gin.Context) {
	data, err := h.stackService.Op(ctx, ctx.Param("name"), ctx.Param("op"))
	if err != nil {
		// 操作失败也带回 compose 输出，前端可展示失败原因
		if data != nil {
			v1.HandleError(ctx, http.StatusInternalServerError, err, data)
			return
		}
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}
