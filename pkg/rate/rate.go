// Package rate 提供基于内存滑动窗口的限流器。
package rate

import (
	"sync"
	"time"
)

// Limiter 是内存滑动窗口限流器。
type Limiter struct {
	mu     sync.Mutex
	tokens []time.Time
	limit  int
	window time.Duration
}

// New 创建限流器：在 window 时间内最多允许 limit 次请求。
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window}
}

// Allow 检查是否允许请求；超过限制时返回 false。
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
		return false
	}
	l.tokens = append(l.tokens, now)
	return true
}

// keyedEntry 是 KeyedLimiter 中的一个独立限流桶及其最近访问时间。
type keyedEntry struct {
	limiter    *Limiter
	lastAccess time.Time
}

// KeyedLimiter 按键（如 "ip|username"）维度的滑动窗口限流器。
// 各键独立计数，惰性清理过期与超容量的键，防止内存无限增长。
type KeyedLimiter struct {
	mu      sync.Mutex
	buckets map[string]*keyedEntry
	limit   int
	window  time.Duration
	maxKeys int // 键数上限，超过时优先清理过期键，仍超则剔除最久未访问
	now     func() time.Time
}

// NewKeyed 创建按键限流器：每个键在 window 内最多 limit 次；
// 键总数超过 maxKeys 时触发清理。
func NewKeyed(limit int, window time.Duration, maxKeys int) *KeyedLimiter {
	return &KeyedLimiter{
		buckets: make(map[string]*keyedEntry),
		limit:   limit,
		window:  window,
		maxKeys: maxKeys,
		now:     time.Now,
	}
}

// Allow 检查指定键是否允许请求。
func (k *KeyedLimiter) Allow(key string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	now := k.now()
	k.sweepLocked(now)
	entry, ok := k.buckets[key]
	if !ok {
		entry = &keyedEntry{limiter: New(k.limit, k.window)}
		k.buckets[key] = entry
	}
	entry.lastAccess = now
	return entry.limiter.Allow()
}

// sweepLocked 清理过期键；键数仍超容量时按最近访问时间剔除最老的。
// 调用方必须持有 k.mu。
func (k *KeyedLimiter) sweepLocked(now time.Time) {
	if len(k.buckets) < k.maxKeys {
		return
	}
	for key, entry := range k.buckets {
		if now.Sub(entry.lastAccess) > k.window {
			delete(k.buckets, key)
		}
	}
	for len(k.buckets) >= k.maxKeys {
		oldestKey, oldestAt := "", now
		for key, entry := range k.buckets {
			if oldestKey == "" || entry.lastAccess.Before(oldestAt) {
				oldestKey, oldestAt = key, entry.lastAccess
			}
		}
		if oldestKey == "" {
			return
		}
		delete(k.buckets, oldestKey)
	}
}
