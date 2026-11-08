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
func (l *Limiter) Allow() bool { return l.allowAt(time.Now()) }

func (l *Limiter) allowAt(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
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
	mu        sync.Mutex
	buckets   map[string]*keyedEntry
	limit     int
	window    time.Duration
	maxKeys   int // 只在新增键时淘汰最久未访问的桶，避免重置已有键额度。
	now       func() time.Time
	nextSweep time.Time
}

// NewKeyed 创建按键限流器：每键独立计数，过期桶定期惰性清理，容量淘汰只用于新增键。
func NewKeyed(limit int, window time.Duration, maxKeys int) *KeyedLimiter {
	if maxKeys < 1 {
		maxKeys = 1
	}
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
		if len(k.buckets) >= k.maxKeys {
			k.evictOldestLocked()
		}
		entry = &keyedEntry{limiter: New(k.limit, k.window)}
		k.buckets[key] = entry
	}
	entry.lastAccess = now
	return entry.limiter.allowAt(now)
}

// sweepLocked 每个窗口至多遍历一次，清理不再依赖容量是否已满。
// 调用方必须持有 k.mu；空闲实例不启动后台清扫 goroutine。
func (k *KeyedLimiter) sweepLocked(now time.Time) {
	if now.Before(k.nextSweep) {
		return
	}
	k.nextSweep = now.Add(k.window)
	for key, entry := range k.buckets {
		if now.Sub(entry.lastAccess) > k.window {
			delete(k.buckets, key)
		}
	}
}

// evictOldestLocked 仅为新键腾出容量，不能在访问已有键时重置它的限流额度。
func (k *KeyedLimiter) evictOldestLocked() {
	var oldestKey string
	var oldest *keyedEntry
	for key, entry := range k.buckets {
		if oldest == nil || entry.lastAccess.Before(oldest.lastAccess) {
			oldestKey, oldest = key, entry
		}
	}
	if oldest != nil {
		delete(k.buckets, oldestKey)
	}
}
