package handler

import (
	"net/http"

	v1 "dockge/api/v1"
	"dockge/internal/service"
	"dockge/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// SettingsHandler 承载设置读写接口；依赖经 do:"" 标签注入。
type SettingsHandler struct {
	Logger          *log.Logger             `do:""`
	SettingsService service.SettingsService `do:""`
}

// NewSettingsHandler 构造设置处理器，由注入容器调用。
func NewSettingsHandler(i do.Injector) (*SettingsHandler, error) {
	return do.MustInvokeStruct[*SettingsHandler](i), nil
}

// GetGlobalEnv 读取 stacks 根目录的 global.env 内容。
func (h *SettingsHandler) GetGlobalEnv(ctx *gin.Context) {
	content, err := h.SettingsService.GetGlobalEnv(ctx)
	if err != nil {
		h.Logger.Error().Err(err).Msg("get global env")
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"globalENV": content})
}

// SetGlobalEnv 写入 global.env 文件。
func (h *SettingsHandler) SetGlobalEnv(ctx *gin.Context) {
	var req v1.SetGlobalEnvRequest
	if err := bindJSON(ctx, &req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
		return
	}
	if err := h.SettingsService.SetGlobalEnv(ctx, req.Content); err != nil {
		h.Logger.Error().Err(err).Msg("set global env")
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// GetPrimaryHostname 读取主主机名设置。
func (h *SettingsHandler) GetPrimaryHostname(ctx *gin.Context) {
	hostname, err := h.SettingsService.GetPrimaryHostname(ctx)
	if err != nil {
		h.Logger.Error().Err(err).Msg("get primary hostname")
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"hostname": hostname})
}

// SetPrimaryHostname 写入主主机名设置。
func (h *SettingsHandler) SetPrimaryHostname(ctx *gin.Context) {
	var req struct {
		Hostname string `json:"hostname"`
	}
	if err := bindJSON(ctx, &req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
		return
	}
	if err := h.SettingsService.SetPrimaryHostname(ctx, req.Hostname); err != nil {
		h.Logger.Error().Err(err).Msg("set primary hostname")
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}
