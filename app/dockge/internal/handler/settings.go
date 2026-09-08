package handler

import (
	"net/http"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// SettingsHandler 承载设置读写接口。
type SettingsHandler struct {
	*Handler
	settingsService service.SettingsService
}

// NewSettingsHandler 构造设置处理器，由注入容器调用。

func NewSettingsHandler(i do.Injector) (*SettingsHandler, error) {
	return &SettingsHandler{
		Handler:         do.MustInvoke[*Handler](i),
		settingsService: do.MustInvoke[service.SettingsService](i),
	}, nil
}

// GetGlobalEnv 读取 stacks 根目录的 global.env 内容。
func (h *SettingsHandler) GetGlobalEnv(ctx *gin.Context) {
	content, err := h.settingsService.GetGlobalEnv(ctx)
	if err != nil {
		v1.HandleError(ctx, http.StatusInternalServerError, err, nil)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"globalENV": content})
}

// SetGlobalEnv 写入 global.env 文件。
func (h *SettingsHandler) SetGlobalEnv(ctx *gin.Context) {
	var req v1.SetGlobalEnvRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.settingsService.SetGlobalEnv(ctx, req.Content); err != nil {
		v1.HandleError(ctx, http.StatusInternalServerError, err, nil)
		return
	}
	v1.HandleSuccess(ctx, nil)
}
