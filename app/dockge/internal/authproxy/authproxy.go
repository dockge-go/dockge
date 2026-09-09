// Package authproxy 实现「受信反向代理头」认证模式：
// Traefik forwardAuth + Authelia/Authentik 等在转发请求时注入 Remote-* 身份头，
// dockge 仅在请求来源命中受信网段时读取这些头，映射到本地用户并签发本地 JWT。
package authproxy

import (
	"net/netip"
	"strings"

	"github.com/spf13/viper"
)

// Config 是受信反代认证模式的配置。
type Config struct {
	Enabled        bool
	TrustedProxies []netip.Prefix
	HeaderUser     string
	HeaderEmail    string
	HeaderName     string
	HeaderGroups   string
	AutoProvision  bool
}

// Identity 是从受信代理头提取出的外部身份。
type Identity struct {
	User   string
	Email  string
	Name   string
	Groups []string
}

// FromViper 从配置读取 auth.proxy 段；未配置时返回关闭状态。
func FromViper(conf *viper.Viper) Config {
	cfg := Config{
		Enabled:       conf.GetBool("auth.proxy.enabled"),
		HeaderUser:    conf.GetString("auth.proxy.headers.user"),
		HeaderEmail:   conf.GetString("auth.proxy.headers.email"),
		HeaderName:    conf.GetString("auth.proxy.headers.name"),
		HeaderGroups:  conf.GetString("auth.proxy.headers.groups"),
		AutoProvision: conf.GetBool("auth.proxy.auto_provision"),
	}
	if cfg.HeaderUser == "" {
		cfg.HeaderUser = "Remote-User"
	}
	if cfg.HeaderEmail == "" {
		cfg.HeaderEmail = "Remote-Email"
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "Remote-Name"
	}
	if cfg.HeaderGroups == "" {
		cfg.HeaderGroups = "Remote-Groups"
	}
	for _, cidr := range conf.GetStringSlice("auth.proxy.trusted_proxies") {
		p, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue // 非法 CIDR 直接忽略，避免一条坏配置导致启动失败
		}
		cfg.TrustedProxies = append(cfg.TrustedProxies, p)
	}
	return cfg
}

// TrustedIP 报告远端地址是否命中受信网段。
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

// FromHeaders 从请求头提取身份；user 头为空即视为无身份。
// headers 以 "Header-Name: value" 键值对传入（由调用方从 http.Header 展开）。
func (c Config) FromHeaders(headers map[string]string) (Identity, bool) {
	id := Identity{User: headers[c.HeaderUser], Email: headers[c.HeaderEmail], Name: headers[c.HeaderName]}
	if id.User == "" {
		return id, false
	}
	for _, g := range strings.Split(headers[c.HeaderGroups], ",") {
		if g = strings.TrimSpace(g); g != "" {
			id.Groups = append(id.Groups, g)
		}
	}
	return id, true
}
