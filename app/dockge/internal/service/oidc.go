package service

import (
	"context"
	"fmt"

	"dockge/app/dockge/internal/authoidc"
	"dockge/pkg/log"

	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// OIDCService 聚合 OIDC 认证的全部运行时组件：
// Provider（发现/验签/PKCE 会话）与一次性票据仓库（回调 → 前端交换）。
type OIDCService struct {
	provider *authoidc.Provider
	tickets  *authoidc.TicketStore
}

// NewOIDCService 构造 OIDC 服务。发现失败时降级为 disabled 并记录告警，
// 本地密码登录不受影响（外部 IdP 故障不应阻断服务启动）。
func NewOIDCService(i do.Injector) (*OIDCService, error) {
	conf := do.MustInvoke[*viper.Viper](i)
	logger := do.MustInvoke[*log.Logger](i)
	provider, err := authoidc.NewProvider(context.Background(), authoidc.FromViper(conf))
	if err != nil {
		logger.Warn().Err(err).Msg("oidc discovery failed, sso disabled until restart")
		provider = &authoidc.Provider{}
	}
	return &OIDCService{provider: provider, tickets: authoidc.NewTicketStore()}, nil
}

// Enabled 报告 OIDC 登录是否可用。
func (s *OIDCService) Enabled() bool {
	return s != nil && s.provider.Enabled()
}

// LoginURL 发起授权码登录，返回 state（写入回调比对 cookie）与 IdP 跳转地址。
func (s *OIDCService) LoginURL() (string, string, error) {
	if !s.Enabled() {
		return "", "", authoidc.ErrDisabled
	}
	return s.provider.BeginLogin()
}

// CompleteLogin 完成 IdP 回调：校验 state、换取并验签 ID Token，
// 返回用户名与是否应授予管理员（admin_groups 命中）。
func (s *OIDCService) CompleteLogin(ctx context.Context, state, code string) (username string, admin bool, err error) {
	if !s.Enabled() {
		return "", false, authoidc.ErrDisabled
	}
	claims, err := s.provider.CompleteLogin(ctx, state, code)
	if err != nil {
		return "", false, err
	}
	username = s.provider.Username(claims)
	if username == "" {
		return "", false, fmt.Errorf("id token 缺少用户名 claim")
	}
	return username, s.provider.IsAdmin(claims), nil
}

// IssueTicket 为已完成外部认证的用户签发一次性票据（60 秒有效）。
func (s *OIDCService) IssueTicket(userID uint) (string, error) {
	return s.tickets.Issue(userID)
}

// ConsumeTicket 消费一次性票据，返回用户 ID。
func (s *OIDCService) ConsumeTicket(code string) (uint, bool) {
	return s.tickets.Consume(code)
}
