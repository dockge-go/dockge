package handler

import (
	"net/http"
	"net/url"

	v1 "dockge/api/v1"
	"dockge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

const oidcCookiePath = "/v1/auth/oidc"

// OIDCHandler 使用固定配置的回调地址，不信任转发头来生成重定向或 Cookie 安全属性。
type OIDCHandler struct {
	OIDC *service.OIDCService `do:""`
}

// NewOIDCHandler 通过统一注入容器装配真实 OIDC 登录链路。
func NewOIDCHandler(i do.Injector) (*OIDCHandler, error) {
	return do.MustInvokeStruct[*OIDCHandler](i), nil
}

// Config 只公开模式，不公开 secret、subject 或外部令牌。
func (h *OIDCHandler) Config(c *gin.Context) {
	mode := "local"
	if h.OIDC.Config.Enabled {
		mode = "oidc"
	}
	v1.HandleSuccess(c, gin.H{"mode": mode})
}

func (h *OIDCHandler) cookie(c *gin.Context, name, value string, age int, sameSite http.SameSite) {
	u, _ := url.Parse(h.OIDC.Config.RedirectURL)
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: oidcCookiePath, MaxAge: age, HttpOnly: true, Secure: u != nil && u.Scheme == "https", SameSite: sameSite})
}

// Login 将浏览器带到提供方授权页面，浏览器绑定 cookie 不暴露给脚本。
func (h *OIDCHandler) Login(c *gin.Context) {
	target, binding, err := h.OIDC.Begin(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Cache-Control", "no-store")
	h.cookie(c, "dockge_oidc_flow", binding, 300, http.SameSiteLaxMode)
	c.Redirect(http.StatusFound, target)
}

// Callback 不将 JWT 放在跳转参数中，只放置 60 秒、一次性 HttpOnly 票据。
func (h *OIDCHandler) Callback(c *gin.Context) {
	binding, _ := c.Cookie("dockge_oidc_flow")
	h.cookie(c, "dockge_oidc_flow", "", -1, http.SameSiteLaxMode)
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Cache-Control", "no-store")
	ticket, err := h.OIDC.Complete(c.Request.Context(), binding, c.Query("state"), c.Query("code"), c.Query("error"))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	h.cookie(c, "dockge_oidc_ticket", ticket, 60, http.SameSiteStrictMode)
	c.Redirect(http.StatusFound, "/?oidc=1")
}

// Session 限制同源 POST 领取票据，避免登录 CSRF；Cookie 领取后立刻删除。
func (h *OIDCHandler) Session(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	expected, _ := url.Parse(h.OIDC.Config.RedirectURL)
	if expected == nil || c.GetHeader("Origin") != expected.Scheme+"://"+expected.Host {
		v1.HandleError(c, 403, &v1.Error{Code: 403, Message: "OIDC 会话领取必须来自配置回调地址的同源页面"}, nil)
		return
	}
	ticket, _ := c.Cookie("dockge_oidc_ticket")
	h.cookie(c, "dockge_oidc_ticket", "", -1, http.SameSiteStrictMode)
	data, err := h.OIDC.Redeem(ticket)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	v1.HandleSuccess(c, data)
}
