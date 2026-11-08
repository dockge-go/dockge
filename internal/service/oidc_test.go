package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "dockge/api/v1"
	"dockge/pkg/config"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// TestOIDCFlowFailure 单次消费发生在交换授权码前，错误或过期事务不能重试绕过校验。
func TestOIDCFlowFailure(t *testing.T) {
	for _, tc := range []struct {
		name, state, code, providerError string
		expired                          bool
		detail                           string
	}{
		{name: "过期", state: "state", code: "code", expired: true, detail: "已过期"},
		{name: "state不匹配", state: "other", code: "code", detail: "state"},
		{name: "授权码缺失", state: "state", detail: "code"},
		{name: "提供方拒绝", state: "state", providerError: "access_denied", detail: "access_denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expiry := time.Now().Add(time.Minute)
			if tc.expired {
				expiry = time.Now().Add(-time.Second)
			}
			s := &OIDCService{Config: config.OIDC{Enabled: true}, flows: map[string]oidcFlow{"binding": {state: "state", expires: expiry}}}
			_, err := s.Complete(context.Background(), "binding", tc.state, tc.code, tc.providerError)
			if err == nil || !strings.Contains(err.Error(), tc.detail) {
				t.Fatalf("error=%v", err)
			}
			if _, err = s.Complete(context.Background(), "binding", tc.state, tc.code, tc.providerError); err == nil || !strings.Contains(err.Error(), "不存在") {
				t.Fatal("flow reused")
			}
		})
	}
}

// TestOIDCTicketExpiry 票据与它包裹的面板会话均须未过期，不能领取已经无效的 JWT。
func TestOIDCTicketExpiry(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		ticketExpiry, sessionExpiry time.Time
		enabled                     bool
		want                        bool
	}{
		{"有效", time.Now().Add(time.Minute), time.Now().Add(time.Hour), true, true},
		{"票据过期", time.Now().Add(-time.Second), time.Now().Add(time.Hour), true, false},
		{"会话过期", time.Now().Add(time.Minute), time.Now().Add(-time.Second), true, false},
		{"模式关闭", time.Now().Add(time.Minute), time.Now().Add(time.Hour), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &OIDCService{Config: config.OIDC{Enabled: tc.enabled}, tickets: map[string]oidcTicket{"ticket": {data: &v1.LoginResponseData{ExpiresAt: tc.sessionExpiry.UnixMilli()}, expires: tc.ticketExpiry}}}
			_, err := s.Redeem("ticket")
			if (err == nil) != tc.want {
				t.Fatalf("error=%v", err)
			}
			if _, err = s.Redeem("ticket"); err == nil {
				t.Fatal("ticket reused")
			}
		})
	}
}

// TestOIDCTicketAtomicRedeem 并发领取一个票据只能成功一次。
func TestOIDCTicketAtomicRedeem(t *testing.T) {
	s := &OIDCService{Config: config.OIDC{Enabled: true}, tickets: map[string]oidcTicket{"ticket": {data: &v1.LoginResponseData{ExpiresAt: time.Now().Add(time.Hour).UnixMilli()}, expires: time.Now().Add(time.Minute)}}}
	var wg sync.WaitGroup
	results := make(chan bool, 12)
	for range 12 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Redeem("ticket"); results <- err == nil }()
	}
	wg.Wait()
	close(results)
	success := 0
	for ok := range results {
		if ok {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successful redeems=%d", success)
	}
}

type oidcDiscoveryTransport func(*http.Request) (*http.Response, error)

func (f oidcDiscoveryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func oidcDiscoveryResponse(req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"issuer":"https://id.example","authorization_endpoint":"https://id.example/authorize","token_endpoint":"https://id.example/token","jwks_uri":"https://id.example/jwks","id_token_signing_alg_values_supported":["RS256"]}`)),
		Request:    req,
	}
}

// blockedOIDCDiscovery 阻塞首个发现请求直到清理，确保失败回归也会回收全部测试 goroutine。
func blockedOIDCDiscovery(t *testing.T) (*OIDCService, *sync.WaitGroup) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	var once sync.Once
	s := &OIDCService{
		Config: config.OIDC{Enabled: true, Issuer: "https://id.example"},
		client: &http.Client{Transport: oidcDiscoveryTransport(func(req *http.Request) (*http.Response, error) {
			once.Do(func() { close(entered) })
			<-req.Context().Done()
			return nil, req.Context().Err()
		})},
		flows: map[string]oidcFlow{"binding": {state: "state", expires: time.Now().Add(time.Minute)}},
		tickets: map[string]oidcTicket{"ticket": {
			data: &v1.LoginResponseData{ExpiresAt: time.Now().Add(time.Hour).UnixMilli()}, expires: time.Now().Add(time.Minute),
		}},
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _, _, _ = s.discover(ctx) }()
	t.Cleanup(func() { cancel(); wg.Wait() })
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("discovery did not enter transport")
	}
	return s, &wg
}

// TestOIDCDiscoveryDoesNotBlockSessionState 网络发现不能阻塞已有票据领取或回调事务的本地校验。
func TestOIDCDiscoveryDoesNotBlockSessionState(t *testing.T) {
	for _, operation := range []string{"redeem", "complete"} {
		t.Run(operation, func(t *testing.T) {
			s, wg := blockedOIDCDiscovery(t)
			result := make(chan error, 1)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if operation == "redeem" {
					_, err := s.Redeem("ticket")
					result <- err
				} else {
					_, err := s.Complete(context.Background(), "binding", "state", "", "")
					result <- err
				}
			}()
			select {
			case err := <-result:
				if operation == "redeem" && err != nil {
					t.Fatalf("redeem: %v", err)
				}
				if operation == "complete" && (err == nil || !strings.Contains(err.Error(), "code")) {
					t.Fatalf("complete: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("session state operation blocked by discovery network request")
			}
		})
	}
}

// TestOIDCDiscoveryWaitingCancellation 等待串行发现的调用必须在首个网络请求结束前响应取消或超时。
func TestOIDCDiscoveryWaitingCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "cancel"
		if deadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			s, wg := blockedOIDCDiscovery(t)
			ctx, cancel := context.WithCancel(context.Background())
			want := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
				want = context.DeadlineExceeded
			}
			defer cancel()
			result := make(chan error, 1)
			wg.Add(1)
			go func() { defer wg.Done(); _, _, err := s.discover(ctx); result <- err }()
			if !deadline {
				cancel()
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("error=%v, want %v", err, want)
				}
			case <-time.After(time.Second):
				t.Fatal("waiting discovery did not respond to context cancellation")
			}
		})
	}
}

// TestOIDCDiscoveryConcurrentInitialization 并发发现只发出一次请求，所有调用共享完整初始化的 provider 与 OAuth 配置。
func TestOIDCDiscoveryConcurrentInitialization(t *testing.T) {
	var requests atomic.Int32
	s := &OIDCService{
		Config: config.OIDC{Issuer: "https://id.example", ClientID: "client"},
		client: &http.Client{Transport: oidcDiscoveryTransport(func(req *http.Request) (*http.Response, error) {
			requests.Add(1)
			return oidcDiscoveryResponse(req), nil
		})},
	}
	type result struct {
		provider *oidc.Provider
		oauth    *oauth2.Config
		err      error
	}
	const callers = 24
	results := make(chan result, callers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			p, o, err := s.discover(context.Background())
			results <- result{p, o, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	first := <-results
	if first.err != nil || first.provider == nil || first.oauth == nil || first.oauth.ClientID != "client" {
		t.Fatalf("first discovery=%+v", first)
	}
	for got := range results {
		if got.err != nil || got.provider != first.provider || got.oauth != first.oauth {
			t.Fatalf("discovery not shared: %+v", got)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("discovery requests=%d, want 1", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := s.discover(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled cached discovery error=%v", err)
	}
}

// TestOIDCDiscoveryRetry 失败或网络请求被取消后必须释放发现权限，不能缓存失败或妨碍后续重试。
func TestOIDCDiscoveryRetry(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "transport_failure"
		if canceled {
			name = "request_canceled"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("discovery transport unavailable")
			if canceled {
				failure = context.Canceled
			}
			var requests atomic.Int32
			s := &OIDCService{
				Config: config.OIDC{Issuer: "https://id.example"},
				client: &http.Client{Transport: oidcDiscoveryTransport(func(req *http.Request) (*http.Response, error) {
					if requests.Add(1) == 1 {
						if canceled {
							cancel()
							return nil, req.Context().Err()
						}
						return nil, failure
					}
					return oidcDiscoveryResponse(req), nil
				})},
			}
			if _, _, err := s.discover(ctx); !errors.Is(err, failure) {
				t.Fatalf("first discovery error=%v, want %v", err, failure)
			}
			if p, o, err := s.discover(context.Background()); err != nil || p == nil || o == nil {
				t.Fatalf("retry provider=%v, oauth=%v, error=%v", p, o, err)
			}
			if got := requests.Load(); got != 2 {
				t.Fatalf("discovery requests=%d, want 2", got)
			}
		})
	}
}
