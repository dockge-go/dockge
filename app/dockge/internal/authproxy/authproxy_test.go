package authproxy

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func viperFromYAML(t *testing.T, yaml string) *viper.Viper {
	t.Helper()
	conf := viper.New()
	conf.SetConfigType("yaml")
	if err := conf.ReadConfig(strings.NewReader(yaml)); err != nil {
		t.Fatalf("read config: %v", err)
	}
	return conf
}

func TestFromViperDefaults(t *testing.T) {
	cfg := FromViper(viperFromYAML(t, "security:\n  auth:\n    proxy: {}\n"))
	if cfg.UsernameHeader != "X-Forwarded-User" {
		t.Errorf("UsernameHeader = %q, want X-Forwarded-User", cfg.UsernameHeader)
	}
	if cfg.AutoProvision {
		t.Error("AutoProvision = true, want false")
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want empty", cfg.TrustedProxies)
	}
}

func TestFromViperFull(t *testing.T) {
	cfg := FromViper(viperFromYAML(t, `
security:
  auth:
    proxy:
      trusted_proxies: ["127.0.0.1/32", "10.0.0.0/8", "not-a-cidr"]
      username_header: X-Auth-User
      auto_provision: true
`))
	if cfg.UsernameHeader != "X-Auth-User" {
		t.Errorf("UsernameHeader = %q, want X-Auth-User", cfg.UsernameHeader)
	}
	if !cfg.AutoProvision {
		t.Error("AutoProvision = false, want true")
	}
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("TrustedProxies = %v, want 2 (invalid CIDR ignored)", cfg.TrustedProxies)
	}
}

func TestTrustedIP(t *testing.T) {
	cfg := FromViper(viperFromYAML(t, `
security:
  auth:
    proxy:
      trusted_proxies: ["127.0.0.1/32", "10.0.0.0/8"]
`))
	cases := []struct {
		remote string
		want   bool
	}{
		{"127.0.0.1:5001", true},
		{"10.1.2.3:443", true},
		{"192.168.1.10:5001", false},
		{"", false},
		{"not-an-addr", false},
	}
	for _, c := range cases {
		if got := cfg.TrustedIP(c.remote); got != c.want {
			t.Errorf("TrustedIP(%q) = %v, want %v", c.remote, got, c.want)
		}
	}
}

func TestTrustedIPFailClosedWithoutCIDRs(t *testing.T) {
	cfg := FromViper(viperFromYAML(t, "security:\n  auth:\n    proxy: {}\n"))
	if cfg.TrustedIP("127.0.0.1:5001") {
		t.Error("TrustedIP accepted loopback with no trusted_proxies configured; must fail closed")
	}
}
