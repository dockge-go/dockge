package handler

import (
	v1 "dockge/api/v1"
	"dockge/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// ResourceHandler 只做传输转换，运行时校验在用例层。
type ResourceHandler struct {
	Service *service.ResourceService `do:""`
}

// NewResourceHandler 注入资源用例。
func NewResourceHandler(i do.Injector) (*ResourceHandler, error) {
	return do.MustInvokeStruct[*ResourceHandler](i), nil
}

// Snapshot 查询各类真实资源。
func (h *ResourceHandler) Snapshot(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	v1.HandleSuccess(c, h.Service.Snapshot(c.Request.Context()))
}

// Action 执行并返回运行时原始输出。
func (h *ResourceHandler) Action(c *gin.Context) {
	var q v1.ResourceActionRequest
	if c.Request.ContentLength != 0 {
		if err := bindJSON(c, &q); err != nil {
			handleServiceError(c, err)
			return
		}
	}
	out, err := h.Service.Action(c.Request.Context(), c.Param("kind"), c.Param("id"), c.Param("action"), q)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	v1.HandleSuccess(c, v1.StackOpResponse{Output: out})
}

// Create 接收资源创建表单，参数由用例层校验。
func (h *ResourceHandler) Create(c *gin.Context) {
	var q v1.ResourceCreateRequest
	if err := bindJSON(c, &q); err != nil {
		handleServiceError(c, err)
		return
	}
	out, err := h.Service.Create(c.Request.Context(), c.Param("kind"), q)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	v1.HandleSuccess(c, v1.StackOpResponse{Output: out})
}

// History 提供真实镜像层。
func (h *ResourceHandler) History(c *gin.Context) {
	rows, err := h.Service.History(c.Request.Context(), c.Param("id"))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	v1.HandleSuccess(c, rows)
}

// System 返回认证后的真实服务器采样，分项错误随数据明确返回。
func (h *ResourceHandler) System(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	v1.HandleSuccess(c, h.Service.System(c.Request.Context()))
}
