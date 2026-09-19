package handler

import (
	"net/http"

	v1 "dockge/api/v1"
	"dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// SettingsHandler 承载设置读写接口。
type SettingsHandler struct {
	*Handler
	settingsService service.SettingsService
	conf            *viper.Viper
}

// NewSettingsHandler 构造设置处理器，由注入容器调用。
func NewSettingsHandler(i do.Injector) (*SettingsHandler, error) {
	return &SettingsHandler{
		Handler:         do.MustInvoke[*Handler](i),
		settingsService: do.MustInvoke[service.SettingsService](i),
		conf:            do.MustInvoke[*viper.Viper](i),
	}, nil
}

// GetGlobalEnv 读取 stacks 根目录的 global.env 内容。
func (h *SettingsHandler) GetGlobalEnv(ctx *gin.Context) {
	content, err := h.settingsService.GetGlobalEnv(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("get global env")
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
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
		h.logger.Error().Err(err).Msg("set global env")
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// GetPrimaryHostname 读取主主机名设置。
func (h *SettingsHandler) GetPrimaryHostname(ctx *gin.Context) {
	hostname, err := h.settingsService.GetPrimaryHostname(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("get primary hostname")
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"hostname": hostname})
}

// SetPrimaryHostname 写入主主机名设置。
func (h *SettingsHandler) SetPrimaryHostname(ctx *gin.Context) {
	var req struct {
		Hostname string `json:"hostname"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.settingsService.SetPrimaryHostname(ctx, req.Hostname); err != nil {
		h.logger.Error().Err(err).Msg("set primary hostname")
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		return
	}
	v1.HandleSuccess(ctx, nil)
}
