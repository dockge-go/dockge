// Package authoidc 实现 OIDC Relying Party 认证模式：
// Authorization Code + PKCE 流程对接外部 IdP，验签 ID Token 后映射到本地用户并签发本地 JWT。
package authoidc

import (
	"context"
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

// ProviderConfig 是单个 OIDC provider 的配置。
type ProviderConfig struct {
	Enabled       bool
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	UsernameClaim string
	AdminGroups   []string
}

// Manager 管理多个 OIDC provider 实例。
type Manager struct {
	mu          sync.RWMutex
	providers   map[string]*Provider
	ticketStore *TicketStore
}

// NewManager 从配置初始化所有启用的 OIDC providers。
func NewManager(ctx context.Context, conf *viper.Viper, baseRedirectURL string) (*Manager, error) {
	m := &Manager{
		providers:   make(map[string]*Provider),
		ticketStore: NewTicketStore(),
	}
	raw := conf.GetStringMap("security.auth.oidc.providers")
	for id, v := range raw {
		pm := v.(map[string]interface{})
		cfg := ProviderConfig{
			Enabled:       true,
			Issuer:        str(pm, "issuer"),
			ClientID:      str(pm, "client_id"),
			ClientSecret:  str(pm, "client_secret"),
			UsernameClaim: str(pm, "username_claim"),
		}
		if cfg.UsernameClaim == "" {
			cfg.UsernameClaim = "preferred_username"
		}
		cfg.Scopes = toStringSlice(pm, "scopes")
		if len(cfg.Scopes) == 0 {
			cfg.Scopes = []string{"openid", "profile", "email"}
		}
		cfg.AdminGroups = toStringSlice(pm, "admin_groups")
		cfg.RedirectURL = fmt.Sprintf("%s/v1/oidc/%s/callback", baseRedirectURL, id)

		p, err := NewProvider(ctx, toLegacyConfig(cfg))
		if err != nil {
			return nil, fmt.Errorf("init oidc provider %q: %w", id, err)
		}
		m.providers[id] = p
	}
	return m, nil
}

func toLegacyConfig(pc ProviderConfig) Config {
	return Config{
		Enabled:       pc.Enabled,
		Issuer:        pc.Issuer,
		ClientID:      pc.ClientID,
		ClientSecret:  pc.ClientSecret,
		RedirectURL:   pc.RedirectURL,
		Scopes:        pc.Scopes,
		UsernameClaim: pc.UsernameClaim,
		AdminGroups:   pc.AdminGroups,
	}
}

func str(m map[string]interface{}, k string) string {
	v, _ := m[k].(string)
	return v
}

func toStringSlice(m map[string]interface{}, k string) []string {
	raw, _ := m[k].([]interface{})
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Providers 返回所有已注册的 provider id 列表。
func (m *Manager) Providers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.providers))
	for id := range m.providers {
		ids = append(ids, id)
	}
	return ids
}

// Get 返回指定 provider，不存在时返回 nil。
func (m *Manager) Get(id string) *Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.providers[id]
}

// TicketStore 返回票据仓库（供 handler 使用）。
func (m *Manager) TicketStore() *TicketStore { return m.ticketStore }
