package handler

import (
	"net/http"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/authoidc"
	"dockge/app/dockge/internal/service"
	"dockge/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// OIDCHandler 处理 OIDC 授权码流：发起认证、接收回调。
type OIDCHandler struct {
	*Handler
	authService service.AuthService
	oidcMgr     *authoidc.Manager
	conf        *viper.Viper
}

// NewOIDCHandler 构造 OIDC 处理器。
func NewOIDCHandler(i do.Injector) (*OIDCHandler, error) {
	return &OIDCHandler{
		Handler:     do.MustInvoke[*Handler](i),
		authService: do.MustInvoke[service.AuthService](i),
		oidcMgr:     do.MustInvoke[*authoidc.Manager](i),
		conf:        do.MustInvoke[*viper.Viper](i),
	}, nil
}

// Auth 发起 OIDC 授权请求：生成 state+PKCE verifier，重定向到 IdP。
func (h *OIDCHandler) Auth(ctx *gin.Context) {
	providerID := ctx.Param("provider")
	provider := h.oidcMgr.Get(providerID)
	if provider == nil || !provider.Enabled() {
		v1.HandleError(ctx, http.StatusNotFound, &v1.Error{Code: 404, Message: "provider not found"}, nil)
		return
	}

	state, verifier, err := provider.BeginLogin()
	if err != nil {
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("oidc_state", state, 300, "/", "", false, true)
	ctx.SetCookie("oidc_verifier", verifier, 300, "/", "", false, true)

	ctx.Redirect(http.StatusFound, verifier)
}

// Callback 接收 OIDC 授权码，换取 ID Token，验签后签发本地 JWT cookie 并重定向至首页。
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

	codeVerifier, _ := ctx.Cookie("oidc_verifier")
	if codeVerifier == "" {
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

	data, err := h.authService.OIDCLogin(ctx, username, false)
	if err != nil {
		h.logger.Warn().Str("username", username).Err(err).Msg("oidc login failed")
		v1.HandleError(ctx, http.StatusInternalServerError, v1.ErrInternalServerError, nil)
		return
	}

	// 清除临时 cookie
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("oidc_state", "", -1, "/", "", false, true)
	ctx.SetCookie("oidc_verifier", "", -1, "/", "", false, true)
	// 写 JWT cookie
	tokenTTL := h.conf.GetInt("security.jwt.token_ttl_hours")
	if tokenTTL <= 0 {
		tokenTTL = 168
	}
	ctx.SetCookie("dockge_token", data.AccessToken, tokenTTL*3600, "/", "", false, true)
	ctx.Set("claims", &jwt.Claims{UserId: data.User.ID})
	ctx.Next()
}
