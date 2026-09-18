package handler

import (
	"net/http"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/authoidc"
	"dockge/app/dockge/internal/security"
	"dockge/app/dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// SettingsHandler 承载设置读写接口。
type SettingsHandler struct {
	*Handler
	settingsService service.SettingsService
	authService     service.AuthService
	conf            *viper.Viper
}

// NewSettingsHandler 构造设置处理器，由注入容器调用。
func NewSettingsHandler(i do.Injector) (*SettingsHandler, error) {
	return &SettingsHandler{
		Handler:         do.MustInvoke[*Handler](i),
		settingsService: do.MustInvoke[service.SettingsService](i),
		authService:     do.MustInvoke[service.AuthService](i),
		conf:            do.MustInvoke[*viper.Viper](i),
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

// AuthConfig 返回当前认证模式与可用 OIDC provider 列表，供登录页渲染 SSO 入口。
func (h *SettingsHandler) AuthConfig(ctx *gin.Context) {
	mode := security.AuthMode(h.conf)
	providers := authoidc.ParseProviders(h.conf)
	type providerInfo struct {
		Label string `json:"label"`
	}
	type providerItem struct {
		ID   string       `json:"id"`
		Info providerInfo `json:"info"`
	}
	pList := make([]providerItem, 0, len(providers))
	for _, id := range authoidc.ProviderIDs(providers) {
		info := providerInfo{Label: "SSO"}
		if l := providers[id].Label; l != "" {
			info.Label = l
		}
		pList = append(pList, providerItem{ID: id, Info: info})
	}
	v1.HandleSuccess(ctx, gin.H{
		"mode":        mode,
		"providers":   pList,
		"disableAuth": h.authService.GetDisableAuth(ctx),
	})
}
