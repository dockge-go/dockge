package handler

import (
	"net/http"
	"net/url"

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
	if !provider.Enabled() {
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
// 失败一律 302 回登录页并携带 oidc_error 查询参数——本端点由浏览器整页
// 跳转进入，返回 JSON 会让用户停留在裸报文页（闭环断点）。
func (h *OIDCHandler) Callback(ctx *gin.Context) {
	providerID := ctx.Param("provider")
	provider := h.oidcMgr.Get(providerID)
	if !provider.Enabled() {
		ctx.Redirect(http.StatusFound, "/login?oidc_error="+url.QueryEscape("provider not found"))
		return
	}

	code := ctx.Query("code")
	state := ctx.Query("state")
	if code == "" || state == "" {
		ctx.Redirect(http.StatusFound, "/login?oidc_error="+url.QueryEscape("缺少授权码或状态参数"))
		return
	}

	storedState, _ := ctx.Cookie("oidc_state")
	if state != storedState {
		ctx.Redirect(http.StatusFound, "/login?oidc_error="+url.QueryEscape("state 校验失败，请重新发起登录"))
		return
	}

	claims, err := provider.CompleteLogin(ctx.Request.Context(), state, code)
	if err != nil {
		h.logger.Warn().Err(err).Str("provider", providerID).Msg("oidc complete failed")
		ctx.Redirect(http.StatusFound, "/login?oidc_error="+url.QueryEscape("IdP 令牌交换或验签失败"))
		return
	}

	username := provider.Username(claims)
	if username == "" {
		ctx.Redirect(http.StatusFound, "/login?oidc_error="+url.QueryEscape("无法从 ID Token 中提取用户名"))
		return
	}

	data, err := h.authService.OIDCLogin(ctx, username, provider.IsAdmin(claims))
	if err != nil {
		h.logger.Warn().Str("username", username).Err(err).Msg("oidc login failed")
		ctx.Redirect(http.StatusFound, "/login?oidc_error="+url.QueryEscape("本地会话签发失败，请重试或使用密码登录"))
		return
	}

	// 清除一次性 state cookie，写 JWT cookie，重定向回首页
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("oidc_state", "", -1, "/", "", false, true)
	ctx.SetCookie("dockge_token", data.AccessToken, int(service.SessionTTL.Seconds()), "/", "", false, true)
	ctx.Redirect(http.StatusFound, "/")
}
