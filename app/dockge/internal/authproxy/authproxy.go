// Package authproxy 实现「受信反向代理头」认证模式：
// Traefik forwardAuth + Authelia/Authentik 等在转发请求时注入身份头，
// dockge 仅在请求来源命中受信网段时读取这些头，映射到本地用户并签发本地 JWT。
// 配置命名空间：security.auth.proxy.{trusted_proxies, username_header, auto_provision}。
package authproxy

import (
	"net/netip"

	"github.com/spf13/viper"
)

// Config 是受信反代认证模式的配置。
type Config struct {
	TrustedProxies []netip.Prefix // 受信来源网段；为空时拒绝一切代理认证（fail-closed）
	UsernameHeader string         // 携带用户名的请求头，默认 X-Forwarded-User
	AutoProvision  bool           // 是否自动创建不存在的本地用户
}

// FromViper 从 security.auth.proxy 段读取配置；未配置时返回关闭状态。
// 非法 CIDR 直接忽略（fail-closed：该来源不会被信任，认证必然失败）。
func FromViper(conf *viper.Viper) Config {
	cfg := Config{
		UsernameHeader: conf.GetString("security.auth.proxy.username_header"),
		AutoProvision:  conf.GetBool("security.auth.proxy.auto_provision"),
	}
	if cfg.UsernameHeader == "" {
		cfg.UsernameHeader = "X-Forwarded-User"
	}
	for _, cidr := range conf.GetStringSlice("security.auth.proxy.trusted_proxies") {
		p, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue // 非法 CIDR 忽略：该来源不被信任，认证失败而非放行
		}
		cfg.TrustedProxies = append(cfg.TrustedProxies, p)
	}
	return cfg
}

// TrustedIP 报告远端地址是否命中受信网段。
// 未配置任何受信网段时一律返回 false，杜绝「无 CIDR 也信任代理头」的开放风险。
func (c Config) TrustedIP(remoteAddr string) bool {
	if len(c.TrustedProxies) == 0 {
		return false
	}
	ip, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		return false
	}
	addr := ip.Addr()
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	for _, p := range c.TrustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
