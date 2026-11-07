package config

import (
	"strings"
	"testing"
)

func TestOIDCConfiguration(t *testing.T) {
	valid := OIDC{Enabled: true, Issuer: "https://id.example/realms/main", ClientID: "dockge", ClientSecret: "secret", RedirectURL: "https://dockge.example/v1/auth/oidc/callback", Subject: "immutable-subject"}
	for _, tc := range []struct {
		name string
		edit func(*OIDC)
		want string
	}{
		{"完整配置", func(c *OIDC) {}, ""},
		{"缺subject", func(c *OIDC) { c.Subject = "" }, "SUBJECT"},
		{"缺secret", func(c *OIDC) { c.ClientSecret = "" }, "CLIENT_SECRET"},
		{"不安全issuer", func(c *OIDC) { c.Issuer = "http://id.example" }, "HTTPS"},
		{"不安全callback", func(c *OIDC) { c.RedirectURL = "http://dockge.example/v1/auth/oidc/callback" }, "HTTPS"},
		{"错误callback路径", func(c *OIDC) { c.RedirectURL = "https://dockge.example/callback" }, "路径"},
		{"callback查询", func(c *OIDC) { c.RedirectURL += "?redirect=evil" }, "查询"},
		{"issuer凭据", func(c *OIDC) { c.Issuer = "https://user:pass@id.example" }, "凭据"},
		{"回环开发", func(c *OIDC) {
			c.Issuer = "http://127.0.0.1:9999"
			c.RedirectURL = "http://localhost:5001/v1/auth/oidc/callback"
		}, ""},
		{"关闭模式", func(c *OIDC) { *c = OIDC{} }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.edit(&c)
			err := c.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want %s", err, tc.want)
			}
		})
	}
}

func TestOIDCEnvironmentInjection(t *testing.T) {
	for key, value := range map[string]string{"ENABLED": "true", "ISSUER": "https://id.example", "CLIENT_ID": "dockge", "CLIENT_SECRET": "secret", "REDIRECT_URL": "https://dockge.example/v1/auth/oidc/callback", "SUBJECT": "only-one"} {
		t.Setenv("DOCKGE_OIDC_"+key, value)
	}
	v, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c := OIDCFrom(v)
	if !c.Enabled || c.Subject != "only-one" || c.ClientSecret != "secret" {
		t.Fatal("env injection failed")
	}
	if v.GetString("stacks_dir") != "/opt/stacks" || v.GetString("data.db.user.dsn") != "/app/data/dockge.db" || v.GetInt("http.port") != 5001 {
		t.Fatal("OIDC changed fixed paths or port")
	}
	t.Setenv("DOCKGE_OIDC_SUBJECT", "")
	if _, err := Load(); err == nil {
		t.Fatal("partial injection silently fell back")
	}
}

// TestOIDCInvalidEnabled 不能将拼错的开关静默解析为 false 并开放本地登录。
func TestOIDCInvalidEnabled(t *testing.T) {
	t.Setenv("DOCKGE_OIDC_ENABLED", "enabled")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DOCKGE_OIDC_ENABLED") {
		t.Fatalf("error=%v", err)
	}
}
