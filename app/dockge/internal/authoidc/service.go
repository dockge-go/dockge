package authoidc

import (
	"context"
	"fmt"

	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// OIDCService 是多 provider OIDC 管理器的事务封装，供 service 层调用。
type OIDCService struct {
	mgr *Manager
}

// NewOIDCService 从注入容器初始化所有 OIDC providers。
func NewOIDCService(i do.Injector) (*OIDCService, error) {
	conf := do.MustInvoke[*viper.Viper](i)
	mode := conf.GetString("security.auth.mode")
	if mode != "oidc" {
		return &OIDCService{}, nil
	}
	baseURL := fmt.Sprintf("http://%s:%s",
		conf.GetString("http.host"),
		conf.GetString("http.port"),
	)
	mgr, err := NewManager(context.Background(), conf, baseURL)
	if err != nil {
		return nil, fmt.Errorf("init oidc manager: %w", err)
	}
	return &OIDCService{mgr: mgr}, nil
}

// Enabled 报告是否配置了至少一个启用的 OIDC provider。
func (s *OIDCService) Enabled() bool { return s.mgr != nil && len(s.mgr.Providers()) > 0 }

// Manager 返回底层 provider 管理器。
func (s *OIDCService) Manager() *Manager { return s.mgr }
