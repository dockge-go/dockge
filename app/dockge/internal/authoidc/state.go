package authoidc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

// stateTTL 是未完成登录（login → callback）的状态保存期。
const stateTTL = 5 * time.Minute

// loginState 是一次进行中的授权码登录：state 绑定 PKCE verifier。
type loginState struct {
	verifier  string
	expiresAt time.Time
}

// BeginLogin 发起新的授权码登录：生成 state 与 PKCE verifier，
// 返回 (state, 授权跳转地址)。state 同时用于绑定浏览器会话（回调时比对 cookie）。
func (p *Provider) BeginLogin() (string, string, error) {
	if !p.Enabled() {
		return "", "", ErrDisabled
	}
	verifier, err := randomToken(48)
	if err != nil {
		return "", "", err
	}
	state, err := randomToken(24)
	if err != nil {
		return "", "", err
	}
	now := time.Now()
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	p.sweepStatesLocked(now)
	p.states[state] = loginState{verifier: verifier, expiresAt: now.Add(stateTTL)}
	return state, p.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), nil
}

// CompleteLogin 完成授权码登录：校验并消费 state，用 code + PKCE verifier
// 换取并验签 ID Token，返回其 claims。
func (p *Provider) CompleteLogin(ctx context.Context, state, code string) (map[string]any, error) {
	if !p.Enabled() {
		return nil, ErrDisabled
	}
	now := time.Now()
	p.stateMu.Lock()
	p.sweepStatesLocked(now)
	st, ok := p.states[state]
	if ok {
		delete(p.states, state) // state 一次性
	}
	p.stateMu.Unlock()
	if !ok || now.After(st.expiresAt) {
		return nil, fmt.Errorf("oidc state 无效或已过期")
	}
	return p.Exchange(ctx, code, st.verifier)
}

// sweepStatesLocked 清理过期登录状态。调用方必须持有 stateMu。
func (p *Provider) sweepStatesLocked(now time.Time) {
	for s, st := range p.states {
		if now.After(st.expiresAt) {
			delete(p.states, s)
		}
	}
}

// randomToken 生成 n 字节随机数的 base64url 编码（PKCE verifier / state）。
func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
