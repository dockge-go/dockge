package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	v1 "dockge/api/v1"
	"dockge/internal/model"
	"dockge/internal/repository"
	"dockge/pkg/config"
	"dockge/pkg/hash"
	"dockge/pkg/jwt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

const oidcFlowTTL = 5 * time.Minute
const oidcMaxFlows = 4096

type oidcFlow struct {
	state, nonce, verifier string
	expires                time.Time
}
type oidcTicket struct {
	data    *v1.LoginResponseData
	expires time.Time
}

// OIDCService 保管短期、单次使用的登录事务；不将外部令牌写进 URL 或数据库。
type OIDCService struct {
	Config   config.OIDC
	repo     *repository.Repository
	jwt      *jwt.JWT
	client   *http.Client
	mu       sync.Mutex
	provider *oidc.Provider
	oauth    *oauth2.Config
	flows    map[string]oidcFlow
	tickets  map[string]oidcTicket
}

// NewOIDCService 不在启动时依赖提供方在线，登录时才发现元数据；失败可重试。
func NewOIDCService(i do.Injector) (*OIDCService, error) {
	c := config.OIDCFrom(do.MustInvoke[*viper.Viper](i))
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &OIDCService{Config: c, repo: do.MustInvoke[*repository.Repository](i), jwt: do.MustInvoke[*jwt.JWT](i), client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("OIDC 端点重定向超过 10 次")
		}
		if err := config.ValidateOIDCURL(req.URL.String()); err != nil {
			return fmt.Errorf("OIDC 端点重定向不安全：%w", err)
		}
		return nil
	}}, flows: make(map[string]oidcFlow), tickets: make(map[string]oidcTicket)}, nil
}

func oidcRandom() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("生成 OIDC 随机校验值：%w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func (s *OIDCService) discover(ctx context.Context) (*oidc.Provider, *oauth2.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.provider != nil {
		return s.provider, s.oauth, nil
	}
	p, err := oidc.NewProvider(oidc.ClientContext(ctx, s.client), s.Config.Issuer)
	if err != nil {
		return nil, nil, fmt.Errorf("OIDC 发现提供方：%w", err)
	}
	ep := p.Endpoint()
	var metadata struct {
		JWKSURL string `json:"jwks_uri"`
	}
	if err := p.Claims(&metadata); err != nil {
		return nil, nil, fmt.Errorf("OIDC 读取发现元数据：%w", err)
	}
	for _, endpoint := range []struct{ name, url string }{{"authorization_endpoint", ep.AuthURL}, {"token_endpoint", ep.TokenURL}, {"jwks_uri", metadata.JWKSURL}} {
		if err := config.ValidateOIDCURL(endpoint.url); err != nil {
			return nil, nil, fmt.Errorf("OIDC 提供方 %s URL 不安全：%w", endpoint.name, err)
		}
	}
	s.provider = p
	s.oauth = &oauth2.Config{ClientID: s.Config.ClientID, ClientSecret: s.Config.ClientSecret, RedirectURL: s.Config.RedirectURL, Endpoint: ep, Scopes: []string{oidc.ScopeOpenID}}
	return p, s.oauth, nil
}

func (s *OIDCService) prune(now time.Time) {
	for k, v := range s.flows {
		if !v.expires.After(now) {
			delete(s.flows, k)
		}
	}
	for k, v := range s.tickets {
		if !v.expires.After(now) {
			delete(s.tickets, k)
		}
	}
}

// Begin 生成授权码 + PKCE/S256 流程；返回仅在 HttpOnly cookie 中保存的浏览器绑定值。
func (s *OIDCService) Begin(ctx context.Context) (string, string, error) {
	if !s.Config.Enabled {
		return "", "", &v1.Error{Code: 404, Message: "OIDC 未启用"}
	}
	_, o, err := s.discover(ctx)
	if err != nil {
		return "", "", err
	}
	binding, err := oidcRandom()
	if err != nil {
		return "", "", err
	}
	state, err := oidcRandom()
	if err != nil {
		return "", "", err
	}
	nonce, err := oidcRandom()
	if err != nil {
		return "", "", err
	}
	verifier := oauth2.GenerateVerifier()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(time.Now())
	if len(s.flows) >= oidcMaxFlows {
		return "", "", &v1.Error{Code: 429, Message: "OIDC 待处理登录过多，请五分钟后重试"}
	}
	s.flows[binding] = oidcFlow{state, nonce, verifier, time.Now().Add(oidcFlowTTL)}
	return o.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), binding, nil
}

// Complete 验证浏览器绑定、state、nonce、签名、issuer、audience、过期时间和唯一 subject。
func (s *OIDCService) Complete(ctx context.Context, binding, state, code, providerError string) (string, error) {
	if !s.Config.Enabled {
		return "", &v1.Error{Code: 404, Message: "OIDC 未启用"}
	}
	s.mu.Lock()
	f, ok := s.flows[binding]
	delete(s.flows, binding)
	s.mu.Unlock()
	if !ok || !f.expires.After(time.Now()) || subtle.ConstantTimeCompare([]byte(f.state), []byte(state)) != 1 {
		return "", &v1.Error{Code: 400, Message: "OIDC 登录事务不存在、已过期或 state 与浏览器不匹配，请重新登录"}
	}
	if providerError != "" {
		return "", &v1.Error{Code: 400, Message: fmt.Sprintf("OIDC 提供方拒绝授权：%s", providerError)}
	}
	if code == "" {
		return "", &v1.Error{Code: 400, Message: "OIDC 回调缺少授权码 code"}
	}
	p, o, err := s.discover(ctx)
	if err != nil {
		return "", err
	}
	token, err := o.Exchange(oidc.ClientContext(ctx, s.client), code, oauth2.VerifierOption(f.verifier))
	if err != nil {
		// OAuth 错误原文可能包含令牌响应体，不把外部敏感值回传给浏览器。
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			return "", fmt.Errorf("OIDC 交换授权码失败：HTTP %d，error=%q，description=%q", re.Response.StatusCode, re.ErrorCode, re.ErrorDescription)
		}
		return "", fmt.Errorf("OIDC 交换授权码失败：%w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return "", fmt.Errorf("OIDC 令牌响应缺少 id_token")
	}
	id, err := p.Verifier(&oidc.Config{ClientID: s.Config.ClientID, SupportedSigningAlgs: []string{oidc.RS256, oidc.ES256}}).Verify(oidc.ClientContext(ctx, s.client), raw)
	if err != nil {
		return "", &v1.Error{Code: 401, Message: fmt.Sprintf("OIDC ID Token 验证失败（签名/issuer/audience/有效期）：%v", err)}
	}
	if id.Issuer != s.Config.Issuer {
		return "", &v1.Error{Code: 401, Message: "OIDC ID Token issuer 与配置不匹配"}
	}
	if subtle.ConstantTimeCompare([]byte(id.Nonce), []byte(f.nonce)) != 1 {
		return "", &v1.Error{Code: 401, Message: "OIDC ID Token nonce 与本次登录不匹配"}
	}
	var claims struct {
		AuthorizedParty string `json:"azp"`
	}
	if err := id.Claims(&claims); err != nil {
		return "", fmt.Errorf("OIDC 读取 azp 声明：%w", err)
	}
	if (len(id.Audience) > 1 && claims.AuthorizedParty == "") || (claims.AuthorizedParty != "" && claims.AuthorizedParty != s.Config.ClientID) {
		return "", &v1.Error{Code: 401, Message: "OIDC ID Token azp 与 client_id 不匹配或多 audience 缺少 azp"}
	}
	if id.Subject != s.Config.Subject {
		return "", &v1.Error{Code: 403, Message: "该 OIDC 身份不是配置的唯一允许身份（subject 不匹配）"}
	}
	user, err := s.repo.SingleUser(ctx)
	if errors.Is(err, repository.ErrNotFound) {
		password, genErr := oidcRandom()
		if genErr != nil {
			return "", genErr
		}
		hashed, hashErr := hash.BcryptHash(password)
		if hashErr != nil {
			return "", fmt.Errorf("初始化 OIDC 单账号密钥：%w", hashErr)
		}
		user = model.DockgeUser{Username: "admin", Nickname: "admin", Password: hashed, Role: model.RoleAdmin, Active: true, Source: "oidc"}
		if err = s.repo.CreateUser(ctx, &user); errors.Is(err, repository.ErrConflict) {
			user, err = s.repo.SingleUser(ctx)
		}
	}
	if err != nil {
		return "", fmt.Errorf("OIDC 读取或初始化单账号：%w", err)
	}
	if !user.Active {
		return "", &v1.Error{Code: 403, Message: "本地唯一账号已停用，OIDC 登录被拒绝"}
	}
	expiry := time.Now().Add(time.Hour).Truncate(time.Second)
	if id.Expiry.Before(expiry) {
		expiry = id.Expiry.Truncate(time.Second)
	}
	local, err := s.jwt.GenToken(user.ID, sessionSecret(&user, s.Config), expiry)
	if err != nil {
		return "", fmt.Errorf("OIDC 签发面板会话：%w", err)
	}
	ticket, err := oidcRandom()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(time.Now())
	if len(s.tickets) >= oidcMaxFlows {
		return "", &v1.Error{Code: 429, Message: "OIDC 待领取会话过多，请一分钟后重试"}
	}
	s.tickets[ticket] = oidcTicket{&v1.LoginResponseData{AccessToken: local, ExpiresAt: expiry.UnixMilli(), User: meData(&user)}, time.Now().Add(time.Minute)}
	return ticket, nil
}

// Redeem 仅消费一次短期 cookie 票据，浏览器通过同源 POST 领取本地会话。
func (s *OIDCService) Redeem(ticket string) (*v1.LoginResponseData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[ticket]
	delete(s.tickets, ticket)
	if !ok || !t.expires.After(time.Now()) || t.data.ExpiresAt <= time.Now().UnixMilli() || !s.Config.Enabled {
		return nil, &v1.Error{Code: 401, Message: "OIDC 会话票据不存在、已使用或已过期，请重新登录"}
	}
	return t.data, nil
}
