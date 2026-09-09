package authoidc

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"dockge/app/dockge/internal/security"

	"github.com/spf13/viper"
)

// Manager 管理多个 OIDC provider 实例。
type Manager struct {
	mu        sync.RWMutex
	providers map[string]*Provider
}

// NewManager 从配置初始化 OIDC providers。
// 仅当 security.auth.mode == "oidc" 时执行网络发现；其他模式返回空管理器，
// 避免无关模式下启动时对 IdP 发起不必要的网络请求。
func NewManager(ctx context.Context, conf *viper.Viper, baseRedirectURL string) (*Manager, error) {
	m := &Manager{providers: make(map[string]*Provider)}
	if security.AuthMode(conf) != security.ModeOIDC {
		return m, nil
	}
	for id, cfg := range ParseProviders(conf) {
		cfg.RedirectURL = fmt.Sprintf("%s/v1/oidc/%s/callback", baseRedirectURL, id)
		p, err := NewProvider(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("init oidc provider %q: %w", id, err)
		}
		m.providers[id] = p
	}
	return m, nil
}

// Providers 返回所有已注册 provider id 的确定性（字典序）列表。
func (m *Manager) Providers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.providers))
	for id := range m.providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Get 返回指定 provider，不存在时返回 nil。
func (m *Manager) Get(id string) *Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.providers[id]
}
