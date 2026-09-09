package authoidc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// ErrDisabled 表示 OIDC 模式未启用（调用方应返回 404/401）。
var ErrDisabled = errors.New("oidc auth disabled")

// Provider 是 OIDC Relying Party：封装端点发现、授权码 + PKCE 登录会话、
// ID Token 验签。仅由 Manager 在 oidc 模式下构造，构造即完成发现。
type Provider struct {
	stateMu  sync.Mutex
	states   map[string]loginState // 进行中的登录：state → PKCE verifier
	cfg      ProviderConfig
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

// NewProvider 发现 IdP 端点并构造 Provider。
func NewProvider(ctx context.Context, cfg ProviderConfig) (*Provider, error) {
	if cfg.Issuer == "" || cfg.ClientID == "" {
		return nil, fmt.Errorf("oidc provider: issuer 与 client_id 不能为空")
	}
	if cfg.UsernameClaim == "" {
		cfg.UsernameClaim = "preferred_username"
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "profile", "email"}
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	idp, err := oidc.NewProvider(discoveryCtx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover oidc issuer %q: %w", cfg.Issuer, err)
	}
	return &Provider{
		cfg: cfg,
		verifier: idp.Verifier(&oidc.Config{
			ClientID: cfg.ClientID,
			// IdP 与本地时钟可能有小幅偏移，放宽 1 分钟
			Now: func() time.Time { return time.Now().Add(time.Minute) },
		}),
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     idp.Endpoint(),
			Scopes:       cfg.Scopes,
		},
		states: make(map[string]loginState),
	}, nil
}

// Enabled 报告 OIDC 是否可用（Manager 仅在 oidc 模式下构造 provider，恒为 true）。
func (p *Provider) Enabled() bool { return p != nil }

// Exchange 用授权码（+PKCE verifier）换取并验签 ID Token，返回其 claims。
func (p *Provider) Exchange(ctx context.Context, code, verifier string) (map[string]any, error) {
	if !p.Enabled() {
		return nil, ErrDisabled
	}
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("oidc token exchange: %w", err)
	}
	idTokenRaw, ok := token.Extra("id_token").(string)
	if !ok || idTokenRaw == "" {
		return nil, fmt.Errorf("oidc token response missing id_token")
	}
	idToken, err := p.verifier.Verify(ctx, idTokenRaw)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode id token claims: %w", err)
	}
	return claims, nil
}

// Username 从 claims 提取用户名。
func (p *Provider) Username(claims map[string]any) string {
	if v, ok := claims[p.cfg.UsernameClaim].(string); ok {
		return v
	}
	return ""
}

// IsAdmin 报告 claims 中的组是否命中管理员组。
func (p *Provider) IsAdmin(claims map[string]any) bool {
	raw, ok := claims[p.cfg.GroupsClaim].([]any)
	if !ok {
		return false
	}
	groups := make([]string, 0, len(raw))
	for _, g := range raw {
		if s, ok := g.(string); ok {
			groups = append(groups, s)
		}
	}
	return p.cfg.adminFor(groups)
}
