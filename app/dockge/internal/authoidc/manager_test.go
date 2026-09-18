package authoidc

import (
	"context"
	"reflect"
	"testing"

	"dockge/app/dockge/internal/testutil"
)

func TestParseProviders(t *testing.T) {
	conf := testutil.ViperFromYAML(t, `
security:
  auth:
    mode: oidc
    oidc:
      providers:
        authentik:
          label: Authentik
          issuer: https://idp.example.com
          client_id: abc
          client_secret: secret
          scopes: ["openid", "profile", "email", "groups"]
          username_claim: preferred_username
          groups_claim: groups
          admin_groups: ["dockge-admins"]
        casdoor:
          label: Casdoor
          issuer: https://casdoor.example.com
          client_id: def
`)
	providers := ParseProviders(conf)
	if len(providers) != 2 {
		t.Fatalf("ParseProviders = %d providers, want 2", len(providers))
	}
	ak := providers["authentik"]
	if ak.Label != "Authentik" || ak.Issuer != "https://idp.example.com" || ak.ClientID != "abc" {
		t.Errorf("authentik config mismatch: %+v", ak)
	}
	if !reflect.DeepEqual(ak.Scopes, []string{"openid", "profile", "email", "groups"}) {
		t.Errorf("authentik scopes = %v", ak.Scopes)
	}
	if !reflect.DeepEqual(ak.AdminGroups, []string{"dockge-admins"}) {
		t.Errorf("authentik admin_groups = %v", ak.AdminGroups)
	}
	if ak.UsernameClaim != "preferred_username" || ak.GroupsClaim != "groups" {
		t.Errorf("authentik claims = %q/%q", ak.UsernameClaim, ak.GroupsClaim)
	}
	if cs := providers["casdoor"]; cs.Label != "Casdoor" || cs.ClientID != "def" {
		t.Errorf("casdoor config mismatch: %+v", cs)
	}
}

func TestProviderIDsSorted(t *testing.T) {
	conf := testutil.ViperFromYAML(t, `
security:
  auth:
    oidc:
      providers:
        zeta:
          issuer: https://z.example.com
          client_id: a
        alpha:
          issuer: https://a.example.com
          client_id: b
`)
	ids := ProviderIDs(ParseProviders(conf))
	if !reflect.DeepEqual(ids, []string{"alpha", "zeta"}) {
		t.Errorf("ProviderIDs = %v, want [alpha zeta]", ids)
	}
}

func TestNewManagerSkipsDiscoveryWhenNotOIDC(t *testing.T) {
	conf := testutil.ViperFromYAML(t, `
security:
  auth:
    mode: jwt
    oidc:
      providers:
        broken:
          issuer: https://nonexistent.invalid
          client_id: x
`)
	m, err := NewManager(context.Background(), conf, "http://localhost:5001")
	if err != nil {
		t.Fatalf("NewManager with mode=jwt must not fail (no discovery): %v", err)
	}
	if len(m.Providers()) != 0 {
		t.Errorf("Providers = %v, want empty when mode != oidc", m.Providers())
	}
}

func TestAdminFor(t *testing.T) {
	cfg := ProviderConfig{AdminGroups: []string{"dockge-admins"}}
	if !cfg.adminFor([]string{"users", "dockge-admins"}) {
		t.Error("adminFor should match member group")
	}
	if cfg.adminFor([]string{"users"}) {
		t.Error("adminFor should reject non-admin groups")
	}
	if (ProviderConfig{}).adminFor([]string{"dockge-admins"}) {
		t.Error("adminFor without admin_groups must never grant admin")
	}
}
