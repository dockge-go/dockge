package rate

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestKeyedLimiterExpiresBelowCapacity(t *testing.T) {
	clock := time.Now()
	k := NewKeyed(1, time.Minute, 4096)
	k.now = func() time.Time { return clock }
	for i := 0; i < 128; i++ {
		if !k.Allow(fmt.Sprint(i)) {
			t.Fatal("新键不应限流")
		}
	}
	clock = clock.Add(2 * time.Minute)
	if !k.Allow("new") {
		t.Fatal("新键不应限流")
	}
	if len(k.buckets) != 1 {
		t.Fatalf("容量未满也应清理过期键，仍有 %d 个", len(k.buckets))
	}
}

func TestKeyedLimiterCapacityDoesNotResetExistingKey(t *testing.T) {
	clock := time.Now()
	k := NewKeyed(1, time.Minute, 2)
	k.now = func() time.Time { return clock }
	if !k.Allow("old") {
		t.Fatal("首次访问不应限流")
	}
	clock = clock.Add(time.Nanosecond)
	if !k.Allow("new") {
		t.Fatal("新键不应限流")
	}
	before := k.buckets["old"]
	clock = clock.Add(time.Nanosecond)
	if k.Allow("old") {
		t.Fatal("满容量访问已有键不能重置额度")
	}
	if before != k.buckets["old"] {
		t.Fatal("已有桶被替换")
	}
	clock = clock.Add(time.Nanosecond)
	if !k.Allow("third") {
		t.Fatal("新键应该可用")
	}
	if len(k.buckets) != 2 || k.buckets["new"] != nil || k.buckets["old"] == nil {
		t.Fatal("新增键只应淘汰最久未访问的桶")
	}
}

func TestKeyedLimiterWindowUsesOneClock(t *testing.T) {
	clock := time.Now()
	k := NewKeyed(1, time.Minute, 10)
	k.now = func() time.Time { return clock }
	if !k.Allow("key") || k.Allow("key") {
		t.Fatal("同一窗口应限制额度")
	}
	// 多次失败刷新 lastAccess，防止仅靠删除整桶恰巧通过窗口测试。
	clock = clock.Add(30 * time.Second)
	if k.Allow("key") {
		t.Fatal("窗口内不能恢复额度")
	}
	clock = clock.Add(31 * time.Second)
	if !k.Allow("key") {
		t.Fatal("过期 token 应在保留原桶的情况下释放")
	}
}

func TestKeyedLimiterEmptyKeyRespectsCapacity(t *testing.T) {
	k := NewKeyed(1, time.Minute, 1)
	if !k.Allow("") || !k.Allow("next") {
		t.Fatal("新键应可用")
	}
	if len(k.buckets) != 1 {
		t.Fatal("空键也必须受容量约束")
	}
}

func TestKeyedLimiterConcurrentLimit(t *testing.T) {
	k := NewKeyed(10, time.Minute, 128)
	var count atomic.Int32
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			if k.Allow("one") {
				count.Add(1)
			}
		})
	}
	wg.Wait()
	if count.Load() != 10 {
		t.Fatalf("并发请求应只放行 10 次，实际 %d", count.Load())
	}
}

func TestKeyedLimiterInvalidCapacityStillBounded(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		k := NewKeyed(1, time.Minute, capacity)
		for _, key := range []string{"a", "b", "c"} {
			_ = k.Allow(key)
		}
		if len(k.buckets) != 1 || k.Allow("c") {
			t.Fatal("非正容量应至少保留一个有效限流桶")
		}
	}
}
