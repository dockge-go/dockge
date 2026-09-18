package handler

import (
	"net/http"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// AuthHandler 承载登录、Setup、2FA、当前用户、改密接口。
type AuthHandler struct {
	*Handler
	authService service.AuthService
}

// NewAuthHandler 构造认证处理器。
func NewAuthHandler(i do.Injector) (*AuthHandler, error) {
	return &AuthHandler{
		Handler:     do.MustInvoke[*Handler](i),
		authService: do.MustInvoke[service.AuthService](i),
	}, nil
}

// Login 处理登录请求。
func (h *AuthHandler) Login(ctx *gin.Context) {
	var req v1.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	clientIP := ctx.ClientIP()
	data, err := h.authService.Login(ctx, &req, clientIP)
	if err != nil {
		v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// Setup 首次安装引导。
func (h *AuthHandler) Setup(ctx *gin.Context) {
	var req v1.SetupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	data, err := h.authService.Setup(ctx, &req)
	if err != nil {
		if e, ok := err.(*v1.Error); ok && e.Code == 409 {
			v1.HandleError(ctx, http.StatusConflict, err, nil)
			return
		}
		v1.HandleError(ctx, http.StatusBadRequest, err, nil)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// NeedSetup 检查是否需要安装引导。
func (h *AuthHandler) NeedSetup(ctx *gin.Context) {
	need, err := h.authService.CheckNeedSetup(ctx)
	if err != nil {
		v1.HandleError(ctx, http.StatusInternalServerError, err, nil)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"needSetup": need})
}

// Me 返回当前用户信息。
func (h *AuthHandler) Me(ctx *gin.Context) {
	data, err := h.authService.Me(ctx, GetUserIdFromCtx(ctx))
	if err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, data)
}

// ChangePassword 修改密码。
func (h *AuthHandler) ChangePassword(ctx *gin.Context) {
	var req v1.ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.authService.ChangePassword(ctx, GetUserIdFromCtx(ctx), &req); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, nil)
}

// GetDisableAuth 获取免登录模式状态。
func (h *AuthHandler) GetDisableAuth(ctx *gin.Context) {
	enabled := h.authService.GetDisableAuth(ctx)
	v1.HandleSuccess(ctx, gin.H{"enabled": enabled})
}

// ToggleDisableAuth 切换免登录模式。
func (h *AuthHandler) ToggleDisableAuth(ctx *gin.Context) {
	uid := GetUserIdFromCtx(ctx)
	var req struct {
		Enable          bool   `json:"enable"`
		CurrentPassword string `json:"currentPassword"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	if err := h.authService.ToggleDisableAuth(ctx, uid, req.Enable, req.CurrentPassword); err != nil {
		handleServiceError(ctx, err)
		return
	}
	v1.HandleSuccess(ctx, gin.H{"enabled": req.Enable})
}

// AutoLogin 免登录模式下自动登录。
func (h *AuthHandler) AutoLogin(ctx *gin.Context) {
	data, err := h.authService.AutoLogin(ctx)
	if err != nil {
		v1.HandleError(ctx, http.StatusUnauthorized, err, nil)
		return
	}
	v1.HandleSuccess(ctx, data)
}
