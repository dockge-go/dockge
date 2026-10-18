package handler

import (
	"fmt"
	"io"
	"net/http"

	v1 "dockge/api/v1"
	"dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// StackHandler 承载 compose 栈的查询、编辑、生命周期与日志接口；依赖经 do:"" 标签注入。
type StackHandler struct {
	StackService service.StackService `do:""`
}

// NewStackHandler 构造栈处理器，由注入容器调用。
func NewStackHandler(i do.Injector) (*StackHandler, error) {
	return do.MustInvokeStruct[*StackHandler](i), nil
}

// List 处理栈列表查询请求。
func (h *StackHandler) List(ctx *gin.Context) {
	filter := ctx.Query("filter")
	data, err := h.StackService.List(ctx, filter)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Get 处理栈详情查询请求。
func (h *StackHandler) Get(ctx *gin.Context) {
	data, err := h.StackService.Get(ctx, ctx.Param("name"))
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Create 处理创建栈请求。
func (h *StackHandler) Create(ctx *gin.Context) {
	var req v1.StackSaveRequest
	if err := bindJSON(ctx, &req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
		return
	}
	if err := h.StackService.Save(ctx, &req, true); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// Update 处理保存栈文件请求。
func (h *StackHandler) Update(ctx *gin.Context) {
	var req v1.StackSaveRequest
	if err := bindJSON(ctx, &req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
		return
	}
	req.Name = ctx.Param("name")
	if err := h.StackService.Save(ctx, &req, false); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// Delete 处理删除栈请求（down + 删目录）。
func (h *StackHandler) Delete(ctx *gin.Context) {
	data, err := h.StackService.Delete(ctx, ctx.Param("name"))
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Op 处理栈生命周期操作（start/stop/restart/down/update）：流式响应，
// compose 输出逐行实时下发（text/plain），失败信息写入流尾。
func (h *StackHandler) Op(ctx *gin.Context) {
	// 语法层错误（非法栈名/未知操作）在写出任何流内容之前返回 4xx：
	// 一旦开始流式响应状态码已固定为 200，客户端错误就再也表达不出来。
	name, op := ctx.Param("name"), ctx.Param("op")
	if !service.StackNamePattern.MatchString(name) {
		v1.HandleError(ctx, http.StatusBadRequest,
			fmt.Errorf("%w: 非法栈名 %q（仅允许小写字母、数字、下划线、连字符）", v1.ErrBadRequest, name), nil)
		return
	}
	if !validStackOp(op) {
		v1.HandleError(ctx, http.StatusBadRequest,
			fmt.Errorf("%w: 不支持的操作 %q（可选 start/stop/restart/down/update）", v1.ErrBadRequest, op), nil)
		return
	}
	w := ctx.Writer
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher := flushWriter{w: w, f: w}
	if err := h.StackService.StreamOp(ctx, flusher, name, op); err != nil {
		// 语义层失败（栈缺失/docker 报错）在流内以文本收尾，前端进度终端呈现
		fmt.Fprintf(flusher, "\n[error] %s\n", err.Error())
	}
}

// validStackOp 是栈生命周期操作的白名单（与 service.StreamOp 保持一致）。
func validStackOp(op string) bool {
	switch op {
	case "start", "stop", "restart", "down", "update":
		return true
	default:
		return false
	}
}

// flushWriter 逐行落盘并立即 flush，保证进度终端的实时性。
type flushWriter struct {
	w io.Writer
	f http.Flusher
}

func (fw flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fl, ok := fw.f.(http.Flusher); ok {
		fl.Flush()
	}
	return n, err
}

// Networks 返回本机网络名列表（编辑器建议）。
func (h *StackHandler) Networks(ctx *gin.Context) {
	data, err := h.StackService.Networks(ctx)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// ServiceOp 处理单服务生命周期操作（start/stop/restart）。
func (h *StackHandler) ServiceOp(ctx *gin.Context) {
	data, err := h.StackService.ServiceOp(ctx, ctx.Param("name"), ctx.Param("service"), ctx.Param("op"))
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

// Stats 处理栈内容器资源占用查询（5s 轮询）。
func (h *StackHandler) Stats(ctx *gin.Context) {
	data, err := h.StackService.Stats(ctx, ctx.Param("name"))
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}
