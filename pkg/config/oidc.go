package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

// OIDC 保持外部身份与本地唯一账号一对一，不允许按邮箱自动注册多个用户。
type OIDC struct {
	Enabled                                              bool
	Issuer, ClientID, ClientSecret, RedirectURL, Subject string
}

// OIDCFrom 读取可由 Kubernetes Secret/ConfigMap 注入的认证配置。
func OIDCFrom(v *viper.Viper) OIDC {
	return OIDC{v.GetBool("oidc.enabled"), v.GetString("oidc.issuer"), v.GetString("oidc.client_id"), v.GetString("oidc.client_secret"), v.GetString("oidc.redirect_url"), v.GetString("oidc.subject")}
}

// Validate 校验完整配置，避免部分注入导致不安全回退到本地登录。
func (c OIDC) Validate() error {
	if !c.Enabled {
		return nil
	}
	for _, f := range []struct{ name, value string }{{"issuer", c.Issuer}, {"client_id", c.ClientID}, {"client_secret", c.ClientSecret}, {"redirect_url", c.RedirectURL}, {"subject", c.Subject}} {
		if strings.TrimSpace(f.value) == "" {
			return fmt.Errorf("OIDC 配置缺少 DOCKGE_OIDC_%s", strings.ToUpper(f.name))
		}
	}
	for _, f := range []struct{ name, value string }{{"issuer", c.Issuer}, {"redirect_url", c.RedirectURL}} {
		if err := ValidateOIDCURL(f.value); err != nil {
			return fmt.Errorf("OIDC %s：%w", f.name, err)
		}
	}
	u, _ := url.Parse(c.RedirectURL)
	if u.Path != "/v1/auth/oidc/callback" {
		return fmt.Errorf("OIDC redirect_url 路径必须为 /v1/auth/oidc/callback，实际为 %q", u.Path)
	}
	return nil
}

// ValidateOIDCURL 仅允许 HTTPS 或回环 HTTP 测试地址，不能禁用签名或 TLS 校验。
func ValidateOIDCURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("无法解析 URL：%w", err)
	}
	if u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("URL 必须有主机且不能含凭据、查询参数或片段")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback())) {
		return fmt.Errorf("URL 必须使用 HTTPS（仅回环地址允许 HTTP）")
	}
	return nil
}
