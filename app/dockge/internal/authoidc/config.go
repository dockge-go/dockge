// Package authoidc 实现 OIDC Relying Party 认证模式：
// Authorization Code + PKCE 流程对接外部 IdP（Authentik / Keycloak / Casdoor 等），
// 验签 ID Token 后映射到本地用户并签发本地 JWT。
// 配置命名空间：security.auth.oidc.providers.<id>.*。
package authoidc

import (
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// ProviderConfig 是单个 OIDC provider 的配置（纯配置解析，不触发网络发现）。
type ProviderConfig struct {
	Label         string // 登录页展示名
	Issuer        string // IdP issuer URL
	ClientID      string
	ClientSecret  string
	RedirectURL   string // 由 Manager 依据 baseURL 推导
	Scopes        []string
	UsernameClaim string // 默认 preferred_username
	GroupsClaim   string // 默认 groups
	AdminGroups   []string
}

// ParseProviders 从 security.auth.oidc.providers 段解析全部 provider 配置。
// 使用 viper.Sub 逐 provider 读取，全程类型安全（无 interface{} 断言）。
func ParseProviders(conf *viper.Viper) map[string]ProviderConfig {
	sub := conf.Sub("security.auth.oidc.providers")
	if sub == nil {
		return nil
	}
	providers := make(map[string]ProviderConfig)
	for _, id := range providerIDs(sub) {
		p := sub.Sub(id)
		if p == nil {
			continue
		}
		providers[id] = ProviderConfig{
			Label:         p.GetString("label"),
			Issuer:        p.GetString("issuer"),
			ClientID:      p.GetString("client_id"),
			ClientSecret:  p.GetString("client_secret"),
			Scopes:        p.GetStringSlice("scopes"),
			UsernameClaim: p.GetString("username_claim"),
			GroupsClaim:   p.GetString("groups_claim"),
			AdminGroups:   p.GetStringSlice("admin_groups"),
		}
	}
	return providers
}

// providerIDs 返回 providers 子树下的顶层键（provider id），按字典序排序，
// 保证 provider 列表确定性（登录页与回调路由顺序稳定）。
func providerIDs(sub *viper.Viper) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, key := range sub.AllKeys() {
		id, _, _ := strings.Cut(key, ".")
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// ProviderIDs 返回配置中 provider id 的确定性（字典序）列表。
func ProviderIDs(providers map[string]ProviderConfig) []string {
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// adminFor 报告 groups 是否命中管理员组；未配置 admin_groups 时不自动授予 admin。
func (c ProviderConfig) adminFor(groups []string) bool {
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
