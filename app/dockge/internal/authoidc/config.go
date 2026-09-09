// Package authoidc 实现 OIDC Relying Party 认证模式：
// Authorization Code + PKCE 流程对接外部 IdP（Authentik / Keycloak / Casdoor 等），
// 验签 ID Token 后映射到本地用户并签发本地 JWT。
package authoidc

import (
	"github.com/spf13/viper"
)

// Config 是 OIDC 认证模式的配置。
type Config struct {
	Enabled       bool
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	UsernameClaim string
	GroupsClaim   string
	AdminGroups   []string
}

// FromViper 从配置读取 auth.oidc 段；未配置时返回关闭状态。
func FromViper(conf *viper.Viper) Config {
	scopes := conf.GetStringSlice("auth.oidc.scopes")
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}
	return Config{
		Enabled:       conf.GetBool("auth.oidc.enabled"),
		Issuer:        conf.GetString("auth.oidc.issuer"),
		ClientID:      conf.GetString("auth.oidc.client_id"),
		ClientSecret:  conf.GetString("auth.oidc.client_secret"),
		RedirectURL:   conf.GetString("auth.oidc.redirect_url"),
		Scopes:        scopes,
		UsernameClaim: conf.GetString("auth.oidc.username_claim"),
		GroupsClaim:   conf.GetString("auth.oidc.groups_claim"),
		AdminGroups:   conf.GetStringSlice("auth.oidc.admin_groups"),
	}
}

// adminFor 报告 groups 是否命中管理员组；未配置 admin_groups 时不自动授予 admin。
func (c Config) adminFor(groups []string) bool {
	if len(c.AdminGroups) == 0 {
		return false
	}
	for _, g := range groups {
		for _, admin := range c.AdminGroups {
			if g == admin {
				return true
			}
		}
	}
	return false
}
