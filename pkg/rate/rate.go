// Package rate 提供基于内存滑动窗口的限流器。
package rate

import (
	"sync"
	"time"
)

// Limiter 是内存滑动窗口限流器。
type Limiter struct {
	mu      sync.Mutex
	tokens  []time.Time
	limit   int
	window  time.Duration
	expired time.Time
}

// New 创建限流器：在 window 时间内最多允许 limit 次请求。
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window}
}

// Allow 检查是否允许请求；超过限制时返回 false 并刷新过期时间。
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	// 清理过期 token
	for len(l.tokens) > 0 && l.tokens[0].Before(cutoff) {
		l.tokens = l.tokens[1:]
	}
	if len(l.tokens) >= l.limit {
		l.expired = l.tokens[0].Add(l.window)
		return false
	}
	l.tokens = append(l.tokens, now)
	return true
}

// WaitTime 返回需要等待多久才能再次请求（0 表示立即可用）。
func (l *Limiter) WaitTime() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.tokens) == 0 {
		return 0
	}
	cutoff := time.Now().Add(-l.window)
	for len(l.tokens) > 0 && l.tokens[0].Before(cutoff) {
		l.tokens = l.tokens[1:]
	}
	if len(l.tokens) < l.limit {
		return 0
	}
	wait := l.tokens[0].Add(l.window).Sub(time.Now())
	if wait < 0 {
		return 0
	}
	return wait
}

// IsExpired 检查当前是否仍在限流窗口内。
func (l *Limiter) IsExpired() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.window)
	for len(l.tokens) > 0 && l.tokens[0].Before(cutoff) {
		l.tokens = l.tokens[1:]
	}
	return len(l.tokens) >= l.limit
}

// Reset 清空所有 token。
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tokens = l.tokens[:0]
}
