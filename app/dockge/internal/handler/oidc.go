package handler

import (
	"net/http"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/authoidc"
	"dockge/app/dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// OIDCHandler 处理 OIDC 授权码流：发起认证、接收回调。
type OIDCHandler struct {
	*Handler
	authService service.AuthService
	oidcMgr     *authoidc.Manager
}

// NewOIDCHandler 构造 OIDC 处理器。
func NewOIDCHandler(i do.Injector) (*OIDCHandler, error) {
	return &OIDCHandler{
		Handler:     do.MustInvoke[*Handler](i),
		authService: do.MustInvoke[service.AuthService](i),
		oidcMgr:     do.MustInvoke[*authoidc.Manager](i),
	}, nil
}

// Auth 发起 OIDC 授权请求：生成 state+PKCE verifier（verifier 仅存服务端），
// 以 oidc_state cookie 绑定浏览器会话，重定向到 IdP 授权页。
func (h *OIDCHandler) Auth(ctx *gin.Context) {
	providerID := ctx.Param("provider")
	provider := h.oidcMgr.Get(providerID)
	if provider == nil || !provider.Enabled() {
		v1.HandleError(ctx, http.StatusNotFound, &v1.Error{Code: 404, Message: "provider not found"}, nil)
		return
	}

	state, authURL, err := provider.BeginLogin()
	if err != nil {
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("oidc_state", state, 300, "/", "", false, true)

	ctx.Redirect(http.StatusFound, authURL)
}

// Callback 接收 OIDC 授权码，换取并验签 ID Token，映射本地用户后
// 签发 httpOnly JWT cookie 并重定向至首页。
func (h *OIDCHandler) Callback(ctx *gin.Context) {
	providerID := ctx.Param("provider")
	provider := h.oidcMgr.Get(providerID)
	if provider == nil || !provider.Enabled() {
		v1.HandleError(ctx, http.StatusNotFound, &v1.Error{Code: 404, Message: "provider not found"}, nil)
		return
	}

	code := ctx.Query("code")
	state := ctx.Query("state")
	if code == "" || state == "" {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}

	storedState, _ := ctx.Cookie("oidc_state")
	if state != storedState {
		v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
		return
	}

	claims, err := provider.CompleteLogin(ctx.Request.Context(), state, code)
	if err != nil {
		h.logger.Warn().Err(err).Str("provider", providerID).Msg("oidc complete failed")
		v1.HandleError(ctx, http.StatusUnauthorized, v1.ErrUnauthorized, nil)
		return
	}

	username := provider.Username(claims)
	if username == "" {
		v1.HandleError(ctx, http.StatusUnauthorized, &v1.Error{Code: 401, Message: "无法从 ID Token 中提取用户名"}, nil)
		return
	}

	data, err := h.authService.OIDCLogin(ctx, username, provider.IsAdmin(claims))
	if err != nil {
		h.logger.Warn().Str("username", username).Err(err).Msg("oidc login failed")
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		return
	}

	// 清除一次性 state cookie，写 JWT cookie，重定向回首页
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("oidc_state", "", -1, "/", "", false, true)
	ctx.SetCookie("dockge_token", data.AccessToken, int(service.SessionTTL.Seconds()), "/", "", false, true)
	ctx.Redirect(http.StatusFound, "/")
}
