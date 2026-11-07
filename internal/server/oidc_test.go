package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

type providerGrant struct{ nonce, challenge, redirect string }
type testOIDCProvider struct {
	server            *httptest.Server
	key               *rsa.PrivateKey
	mu                sync.Mutex
	grants            map[string]providerGrant
	claims            func(jwtlib.MapClaims)
	badSignature      bool
	tokenError        bool
	discoveryFailures int
	jwksOverride      string
	omitIDToken       bool
}

// newOIDCProvider 测试真实 discovery、授权码交换、JWKS 和签名验证，不跳过协议检查。
func newOIDCProvider(t *testing.T) *testOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &testOIDCProvider{key: key, grants: map[string]providerGrant{}}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			p.mu.Lock()
			defer p.mu.Unlock()
			if p.discoveryFailures > 0 {
				p.discoveryFailures--
				w.WriteHeader(503)
				_, _ = w.Write([]byte(`{"error":"temporarily_unavailable"}`))
				return
			}
			jwks := p.server.URL + "/jwks"
			if p.jwksOverride != "" {
				jwks = p.jwksOverride
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": p.server.URL, "authorization_endpoint": p.server.URL + "/authorize", "token_endpoint": p.server.URL + "/token", "jwks_uri": jwks, "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": "test", "n": base64.RawURLEncoding.EncodeToString(p.key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.key.E)).Bytes())}}})
		case "/token":
			_ = r.ParseForm()
			user, secret, basic := r.BasicAuth()
			if !basic {
				user = r.Form.Get("client_id")
				secret = r.Form.Get("client_secret")
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			g, ok := p.grants[r.Form.Get("code")]
			challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if !ok || user != "dockge" || secret != "test-secret" || r.Form.Get("redirect_uri") != g.redirect || base64.RawURLEncoding.EncodeToString(challenge[:]) != g.challenge || p.tokenError {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"PKCE or code validation failed"}`))
				return
			}
			delete(p.grants, r.Form.Get("code"))
			claims := jwtlib.MapClaims{"iss": p.server.URL, "aud": "dockge", "sub": "single-subject", "nonce": g.nonce, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
			if p.claims != nil {
				p.claims(claims)
			}
			key := p.key
			if p.badSignature {
				key, _ = rsa.GenerateKey(rand.Reader, 2048)
			}
			token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
			token.Header["kid"] = "test"
			signed, e := token.SignedString(key)
			if e != nil {
				t.Error(e)
			}
			response := map[string]any{"access_token": "provider-access-token", "token_type": "Bearer", "expires_in": 3600}
			if !p.omitIDToken {
				response["id_token"] = signed
			}
			_ = json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(p.server.Close)
	return p
}

func oidcRequest(e *gin.Engine, method, path string, cookies []*http.Cookie, origin, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	e.ServeHTTP(rr, req)
	return rr
}

func oidcStart(t *testing.T, p *testOIDCProvider, e *gin.Engine) (*httptest.ResponseRecorder, url.Values, string) {
	t.Helper()
	r := oidcRequest(e, "GET", "/v1/auth/oidc/login", nil, "", "")
	if r.Code != 302 {
		t.Fatalf("start %d %s", r.Code, r.Body.String())
	}
	u, err := url.Parse(r.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("scope") != "openid" || q.Get("code_challenge_method") != "S256" || q.Get("nonce") == "" || q.Get("state") == "" {
		t.Fatalf("incomplete OIDC request: %s", u)
	}
	code := fmt.Sprintf("code-%s", q.Get("state"))
	p.mu.Lock()
	p.grants[code] = providerGrant{q.Get("nonce"), q.Get("code_challenge"), q.Get("redirect_uri")}
	p.mu.Unlock()
	return r, q, code
}

func TestOIDCProtocolRoundTrip(t *testing.T) {
	p := newOIDCProvider(t)
	setOIDCEnvironment(t, p, "http://localhost:5001/v1/auth/oidc/callback")
	e := newTestServer(t)
	if code, payload := call(t, e, "GET", "/v1/auth/config", "", nil); code != 200 || dataString(payload, "mode") != "oidc" {
		t.Fatalf("mode %d %v", code, payload)
	}
	if code, payload := call(t, e, "GET", "/v1/setup/need", "", nil); code != 200 || payload["data"].(map[string]any)["needSetup"] != false {
		t.Fatal("OIDC requested local setup")
	}
	for _, path := range []string{"/v1/setup", "/v1/login", "/v1/auto-login"} {
		if code, _ := call(t, e, "POST", path, "", map[string]string{"username": "admin", "password": "Pass123"}); code < 400 {
			t.Fatalf("local bypass %s", path)
		}
	}
	start, q, code := oidcStart(t, p, e)
	for _, c := range start.Result().Cookies() {
		if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/v1/auth/oidc" {
			t.Fatal("insecure flow cookie")
		}
	}
	callback := "/v1/auth/oidc/callback?state=" + q.Get("state") + "&code=" + url.QueryEscape(code)
	done := oidcRequest(e, "GET", callback, start.Result().Cookies(), "", "")
	if done.Code != 302 || done.Header().Get("Location") != "/?oidc=1" {
		t.Fatalf("callback %d %s", done.Code, done.Body.String())
	}
	if strings.Contains(done.Header().Get("Location"), "token") {
		t.Fatal("token leaked in URL")
	}
	cookies := done.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "dockge_oidc_ticket" && (!c.HttpOnly || c.MaxAge != 60 || c.SameSite != http.SameSiteStrictMode) {
			t.Fatal("insecure ticket cookie")
		}
	}
	evil := oidcRequest(e, "POST", "/v1/auth/oidc/session", cookies, "https://evil.example", "")
	if evil.Code != 403 {
		t.Fatal("cross-origin redeem allowed")
	}
	session := oidcRequest(e, "POST", "/v1/auth/oidc/session", cookies, "http://localhost:5001", "")
	if session.Code != 200 {
		t.Fatalf("session %d %s", session.Code, session.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(session.Body.Bytes(), &payload)
	token := dataString(payload, "accessToken")
	if token == "" {
		t.Fatal("no local JWT")
	}
	var localClaims jwtlib.MapClaims
	if _, _, err := jwtlib.NewParser().ParseUnverified(token, &localClaims); err != nil {
		t.Fatal(err)
	}
	localExpiry, err := localClaims.GetExpirationTime()
	if err != nil || localExpiry == nil || payload["data"].(map[string]any)["expiresAt"] != float64(localExpiry.Time.UnixMilli()) {
		t.Fatal("OIDC 响应有效期与面板 JWT 不一致")
	}
	if strings.Contains(session.Body.String(), "provider-access-token") || strings.Contains(session.Body.String(), "test-secret") {
		t.Fatal("external credential leaked")
	}
	if rr := oidcRequest(e, "GET", "/v1/me", nil, "", token); rr.Code != 200 {
		t.Fatalf("session unusable %s", rr.Body.String())
	}
	if rr := oidcRequest(e, "POST", "/v1/auth/oidc/session", cookies, "http://localhost:5001", ""); rr.Code != 401 {
		t.Fatal("ticket replay allowed")
	}
	if rr := oidcRequest(e, "GET", callback, start.Result().Cookies(), "", ""); rr.Code != 400 {
		t.Fatal("callback replay allowed")
	}
	for _, tc := range []struct {
		name                     string
		change                   func(jwtlib.MapClaims)
		badSignature, tokenError bool
		omitIDToken              bool
		state                    string
		noCookie                 bool
		want                     int
		detail                   string
	}{
		{name: "错误issuer", change: func(c jwtlib.MapClaims) { c["iss"] = "https://evil.example" }, want: 401, detail: "issuer"},
		{name: "错误audience", change: func(c jwtlib.MapClaims) { c["aud"] = "another-client" }, want: 401, detail: "audience"},
		{name: "过期", change: func(c jwtlib.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, want: 401, detail: "expired"},
		{name: "错误nonce", change: func(c jwtlib.MapClaims) { c["nonce"] = "wrong" }, want: 401, detail: "nonce"},
		{name: "第二个身份", change: func(c jwtlib.MapClaims) { c["sub"] = "other-user" }, want: 403, detail: "subject"},
		{name: "错误azp", change: func(c jwtlib.MapClaims) { c["azp"] = "evil" }, want: 401, detail: "azp"},
		{name: "多aud缺azp", change: func(c jwtlib.MapClaims) { c["aud"] = []string{"dockge", "other"} }, want: 401, detail: "azp"},
		{name: "错误签名", badSignature: true, want: 401, detail: "signature"},
		{name: "缺少IDToken", omitIDToken: true, want: 500, detail: "id_token"},
		{name: "交换失败", tokenError: true, want: 500, detail: "invalid_grant"},
		{name: "错误state", state: "wrong", want: 400, detail: "state"},
		{name: "无浏览器cookie", noCookie: true, want: 400, detail: "浏览器"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p.mu.Lock()
			p.claims = tc.change
			p.badSignature = tc.badSignature
			p.tokenError = tc.tokenError
			p.omitIDToken = tc.omitIDToken
			p.mu.Unlock()
			start, q, code := oidcStart(t, p, e)
			state := q.Get("state")
			if tc.state != "" {
				state = tc.state
			}
			cookies := start.Result().Cookies()
			if tc.noCookie {
				cookies = nil
			}
			rr := oidcRequest(e, "GET", "/v1/auth/oidc/callback?state="+state+"&code="+url.QueryEscape(code), cookies, "", "")
			if rr.Code != tc.want || !strings.Contains(rr.Body.String(), tc.detail) {
				t.Fatalf("%d %s want %d %s", rr.Code, rr.Body.String(), tc.want, tc.detail)
			}
		})
	}
	p.mu.Lock()
	p.claims = nil
	p.badSignature = false
	p.tokenError = false
	p.omitIDToken = false
	p.mu.Unlock()
	// 第二次合法登录复用唯一账号，不触发新增用户。
	start, q, code = oidcStart(t, p, e)
	done = oidcRequest(e, "GET", "/v1/auth/oidc/callback?state="+q.Get("state")+"&code="+url.QueryEscape(code), start.Result().Cookies(), "", "")
	if done.Code != 302 {
		t.Fatalf("second login %s", done.Body.String())
	}
	if rr := oidcRequest(e, "GET", "/v1/users", nil, "", token); rr.Code != 404 {
		t.Fatal("user management resurrected")
	}
}

func setOIDCEnvironment(t *testing.T, p *testOIDCProvider, callback string) {
	t.Helper()
	for k, v := range map[string]string{"ENABLED": "true", "ISSUER": p.server.URL, "CLIENT_ID": "dockge", "CLIENT_SECRET": "test-secret", "SUBJECT": "single-subject", "REDIRECT_URL": callback} {
		t.Setenv("DOCKGE_OIDC_"+k, v)
	}
}

// TestOIDCDiscoveryRetry 发现失败不缓存失败结果，不安全 JWKS 不能进入验签链。
func TestOIDCDiscoveryRetry(t *testing.T) {
	p := newOIDCProvider(t)
	p.discoveryFailures = 1
	setOIDCEnvironment(t, p, "https://dockge.example/v1/auth/oidc/callback")
	e := newTestServer(t)
	if r := oidcRequest(e, "GET", "/v1/auth/oidc/login", nil, "", ""); r.Code != 500 || !strings.Contains(r.Body.String(), "503") {
		t.Fatalf("discovery=%d %s", r.Code, r.Body.String())
	}
	p.mu.Lock()
	p.jwksOverride = "http://unsafe.example/jwks"
	p.mu.Unlock()
	if r := oidcRequest(e, "GET", "/v1/auth/oidc/login", nil, "", ""); r.Code != 500 || !strings.Contains(r.Body.String(), "jwks_uri") {
		t.Fatalf("JWKS=%d %s", r.Code, r.Body.String())
	}
	p.mu.Lock()
	p.jwksOverride = ""
	p.mu.Unlock()
	start, q, code := oidcStart(t, p, e)
	for _, c := range start.Result().Cookies() {
		if !c.Secure {
			t.Fatal("HTTPS flow cookie not Secure")
		}
	}
	done := oidcRequest(e, "GET", "/v1/auth/oidc/callback?state="+q.Get("state")+"&code="+url.QueryEscape(code), start.Result().Cookies(), "", "")
	if done.Code != 302 {
		t.Fatal(done.Body.String())
	}
	for _, c := range done.Result().Cookies() {
		if !c.Secure {
			t.Fatal("HTTPS callback cookie not Secure")
		}
	}
	session := oidcRequest(e, "POST", "/v1/auth/oidc/session", done.Result().Cookies(), "https://dockge.example", "")
	if session.Code != 200 || session.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("session=%d %s", session.Code, session.Body.String())
	}
}
