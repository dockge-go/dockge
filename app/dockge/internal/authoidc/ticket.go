package authoidc

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// ticketTTL 是一次性票据的有效期：OIDC 回调 → 前端 exchange 的窗口。
const ticketTTL = 60 * time.Second

// TicketStore 保管回调签发的一次性登录票据（内存态，重启即失效）。
// 票据仅可消费一次，过期自动作废。
type TicketStore struct {
	mu      sync.Mutex
	tickets map[string]oidcTicket
}

type oidcTicket struct {
	userID    uint
	expiresAt time.Time
}

// NewTicketStore 构造空票据仓库。
func NewTicketStore() *TicketStore {
	return &TicketStore{tickets: make(map[string]oidcTicket)}
}

// Issue 为用户签发一张一次性票据，返回票据码。
func (s *TicketStore) Issue(userID uint) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := hex.EncodeToString(buf)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(now)
	s.tickets[code] = oidcTicket{userID: userID, expiresAt: now.Add(ticketTTL)}
	return code, nil
}

// Consume 消费票据：有效则返回用户 ID 并立即作废该票据。
func (s *TicketStore) Consume(code string) (uint, bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(now)
	t, ok := s.tickets[code]
	if !ok || now.After(t.expiresAt) {
		return 0, false
	}
	delete(s.tickets, code)
	return t.userID, true
}

// sweepLocked 清理过期票据。调用方必须持有 s.mu。
func (s *TicketStore) sweepLocked(now time.Time) {
	for code, t := range s.tickets {
		if now.After(t.expiresAt) {
			delete(s.tickets, code)
		}
	}
}
