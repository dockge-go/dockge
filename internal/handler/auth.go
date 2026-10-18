package handler

import (
	"net/http"
	"time"

	v1 "dockge/api/v1"
	"dockge/internal/service"
	"dockge/pkg/log"
	"dockge/pkg/rate"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// AuthHandler 承载登录、Setup、2FA、当前用户、改密接口；依赖经 do:"" 标签注入。
type AuthHandler struct {
	Logger      *log.Logger         `do:""`
	AuthService service.AuthService `do:""`
}

// NewAuthHandler 构造认证处理器。
func NewAuthHandler(i do.Injector) (*AuthHandler, error) {
	return do.MustInvokeStruct[*AuthHandler](i), nil
}

// loginLimiter 按 IP 限制登录尝试频率（防在线暴力破解）。
var loginLimiter = rate.NewKeyed(10, time.Minute, 10_000)

// Login 处理登录请求。
func (h *AuthHandler) Login(ctx *gin.Context) {
	if !loginLimiter.Allow(ctx.ClientIP()) {
		v1.HandleError(ctx, http.StatusTooManyRequests,
			&v1.Error{Code: http.StatusTooManyRequests, Message: "尝试过于频繁，请一分钟后再试"}, nil)
		return
	}
	var req v1.LoginRequest
	if err := bindJSON(ctx, &req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
		return
	}
	clientIP := ctx.ClientIP()
	data, err := h.AuthService.Login(ctx, &req, clientIP)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Setup 首次安装引导。
func (h *AuthHandler) Setup(ctx *gin.Context) {
	var req v1.SetupRequest
	if err := bindJSON(ctx, &req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
		return
	}
	data, err := h.AuthService.Setup(ctx, &req)
	if err != nil {
		// service 已按业务码自备具体文案（409 已初始化 / 400 密码强度 / 500 带原因），
		// 统一走 handleServiceError 按 code 映射，不再手工猜 HTTP 码
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// NeedSetup 检查是否需要安装引导。
func (h *AuthHandler) NeedSetup(ctx *gin.Context) {
	need, err := h.AuthService.CheckNeedSetup(ctx)
	if err != nil {
		h.Logger.Error().Err(err).Msg("check need setup")
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"needSetup": need})
}

// Me 返回当前用户信息。
func (h *AuthHandler) Me(ctx *gin.Context) {
	data, err := h.AuthService.Me(ctx, GetUserIDFromCtx(ctx))
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// ChangePassword 修改密码。
func (h *AuthHandler) ChangePassword(ctx *gin.Context) {
	var req v1.ChangePasswordRequest
	if err := bindJSON(ctx, &req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
		return
	}
	if err := h.AuthService.ChangePassword(ctx, GetUserIDFromCtx(ctx), &req); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// GetDisableAuth 获取免登录模式状态。
func (h *AuthHandler) GetDisableAuth(ctx *gin.Context) {
	enabled := h.AuthService.GetDisableAuth(ctx)
	v1.HandleSuccess(ctx, gin.H{"enabled": enabled})
}

// ToggleDisableAuth 切换免登录模式。
func (h *AuthHandler) ToggleDisableAuth(ctx *gin.Context) {
	uid := GetUserIDFromCtx(ctx)
	var req struct {
		Enable          bool   `json:"enable"`
		CurrentPassword string `json:"currentPassword"`
	}
	if err := bindJSON(ctx, &req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
		return
	}
	if err := h.AuthService.ToggleDisableAuth(ctx, uid, req.Enable, req.CurrentPassword); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"enabled": req.Enable})
}

// AutoLogin 免登录模式下自动登录。
func (h *AuthHandler) AutoLogin(ctx *gin.Context) {
	data, err := h.AuthService.AutoLogin(ctx)
	if err != nil {
		v1.HandleError(ctx, http.StatusUnauthorized, err, nil)
		return
	}
	v1.HandleSuccess(ctx, data)
}
