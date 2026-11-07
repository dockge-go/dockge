package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "dockge/api/v1"
	"dockge/pkg/config"
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
