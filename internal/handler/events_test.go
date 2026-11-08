package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func receiveEventSample(t *testing.T, out <-chan eventSample) eventSample {
	t.Helper()
	select {
	case sample := <-out:
		return sample
	case <-time.After(2 * time.Second):
		t.Fatal("等待共享采样超时")
		return eventSample{}
	}
}

func receiveEventNumber(t *testing.T, out <-chan eventSample, want int) eventSample {
	t.Helper()
	for {
		sample := receiveEventSample(t, out)
		if sample.err != nil {
			t.Fatal(sample.err)
		}
		var value int
		if err := json.Unmarshal(sample.data, &value); err != nil {
			t.Fatal(err)
		}
		if value == want {
			return sample
		}
		if value > want {
			t.Fatalf("期待采样 %d，实际 %d", want, value)
		}
	}
}

func triggerEventSample(s *eventSampler) {
	s.mu.Lock()
	s.notify()
	s.mu.Unlock()
}

func waitEventSamplerStopped(t *testing.T, s *eventSampler) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		stopped := !s.running && s.cancel == nil && len(s.clients) == 0
		s.mu.Unlock()
		if stopped {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("最后订阅退出后采样线程未回收")
}

func TestEventSamplerSharesSamplingAndJSON(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	s := newEventSampler(time.Hour, func(context.Context) any {
		n := calls.Add(1)
		if n == 1 {
			close(started)
			<-release
		}
		return n
	})
	first, stop := s.subscribe()
	t.Cleanup(stop)
	<-started
	clients := []<-chan eventSample{first}
	stops := []func(){stop}
	for range 7 {
		out, stop := s.subscribe()
		t.Cleanup(stop)
		clients = append(clients, out)
		stops = append(stops, stop)
	}
	close(release)
	var encoded []byte
	for _, out := range clients {
		sample := receiveEventNumber(t, out, 2)
		if encoded == nil {
			encoded = sample.data
		}
		if &encoded[0] != &sample.data[0] {
			t.Fatal("同一轮采样必须共享同一份不可变 JSON")
		}
	}
	triggerEventSample(s)
	for _, out := range clients {
		receiveEventNumber(t, out, 3)
	}
	if calls.Load() != 3 {
		t.Fatalf("8 个连接同一轮只能采样一次，实际 %d", calls.Load())
	}
	for _, stop := range stops[:7] {
		stop()
	}
	triggerEventSample(s)
	receiveEventNumber(t, clients[7], 4)
	stops[7]()
	waitEventSamplerStopped(t, s)
}

func TestEventSamplerSlowSubscriberKeepsOnlyLatest(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	s := newEventSampler(time.Hour, func(context.Context) any {
		n := calls.Add(1)
		if n == 1 {
			close(started)
			<-release
		}
		return n
	})
	slow, stopSlow := s.subscribe()
	t.Cleanup(stopSlow)
	<-started
	fast, stopFast := s.subscribe()
	t.Cleanup(stopFast)
	close(release)
	receiveEventNumber(t, fast, 2)
	for n := 3; n <= 50; n++ {
		triggerEventSample(s)
		receiveEventNumber(t, fast, n)
	}
	if len(slow) != 1 {
		t.Fatalf("慢连接待发帧必须有界，实际 %d", len(slow))
	}
	receiveEventNumber(t, slow, 50)
	stopSlow()
	stopFast()
	if len(slow) != 0 || len(fast) != 0 {
		t.Fatal("退订必须释放待发数据")
	}
	waitEventSamplerStopped(t, s)
}

func TestEventSamplerFreshSubscriptionExcludesInflightSnapshot(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var value atomic.Int32
	var calls atomic.Int32
	value.Store(1)
	s := newEventSampler(time.Hour, func(context.Context) any {
		n := calls.Add(1)
		result := value.Load()
		if n == 1 {
			close(started)
			<-release
		}
		return result
	})
	old, stopOld := s.subscribe()
	t.Cleanup(stopOld)
	<-started
	// 模拟用户完成删除/保存后重连，旧查询已经读取了操作前的数据。
	value.Store(2)
	fresh, stopFresh := s.subscribe()
	t.Cleanup(stopFresh)
	close(release)
	if sample := receiveEventSample(t, fresh); string(sample.data) != "2" {
		t.Fatalf("刷新首帧不能复用旧在途数据：%s / %v", sample.data, sample.err)
	}
	receiveEventNumber(t, old, 2)
	if calls.Load() != 2 {
		t.Fatalf("重连后应追加一轮真实查询，实际 %d", calls.Load())
	}
	stopOld()
	stopFresh()
	waitEventSamplerStopped(t, s)
}

func TestEventSamplerLastUnsubscribeCancelsWithoutOverlappingRestart(t *testing.T) {
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls, active, peak atomic.Int32
	s := newEventSampler(time.Hour, func(ctx context.Context) any {
		n := calls.Add(1)
		current := active.Add(1)
		for previous := peak.Load(); current > previous && !peak.CompareAndSwap(previous, current); previous = peak.Load() {
		}
		defer active.Add(-1)
		if n == 1 {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-release
		}
		return n
	})
	old, stopOld := s.subscribe()
	t.Cleanup(stopOld)
	<-started
	stopOld()
	<-canceled
	fresh, stopFresh := s.subscribe()
	t.Cleanup(stopFresh)
	stopOld() // 旧请求重复清理不能取消新请求。
	close(release)
	receiveEventNumber(t, fresh, 2)
	if len(old) != 0 {
		t.Fatal("取消的采样不得向旧连接发布")
	}
	if peak.Load() != 1 {
		t.Fatalf("旧采样回收前不得启动新一轮：peak=%d", peak.Load())
	}
	stopFresh()
	waitEventSamplerStopped(t, s)
}

func TestEventSamplerEncodingErrorHasDetailsAndCanRecover(t *testing.T) {
	var calls atomic.Int32
	s := newEventSampler(time.Hour, func(context.Context) any {
		if calls.Add(1) == 1 {
			return make(chan int)
		}
		return 2
	})
	out, stop := s.subscribe()
	t.Cleanup(stop)
	sample := receiveEventSample(t, out)
	if sample.err == nil || !strings.Contains(sample.err.Error(), "unsupported type: chan int") {
		t.Fatalf("必须保留编码失败原因：%v", sample.err)
	}
	triggerEventSample(s)
	receiveEventNumber(t, out, 2)
	stop()
	waitEventSamplerStopped(t, s)
}

func TestEventSamplerConcurrentSubscribeUnsubscribe(t *testing.T) {
	var active, peak atomic.Int32
	s := newEventSampler(time.Millisecond, func(ctx context.Context) any {
		current := active.Add(1)
		for previous := peak.Load(); current > previous && !peak.CompareAndSwap(previous, current); previous = peak.Load() {
		}
		defer active.Add(-1)
		return ctx.Err() == nil
	})
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 100 {
				_, stop := s.subscribe()
				stop()
				stop()
			}
		})
	}
	wg.Wait()
	waitEventSamplerStopped(t, s)
	if peak.Load() > 1 {
		t.Fatalf("并发重连启动了重复采样：peak=%d", peak.Load())
	}
}

func TestEventsCanceledRequestDoesNotSample(t *testing.T) {
	h := &EventsHandler{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/events", nil).WithContext(ctx)
	h.Events(c)
	if h.systems != nil || h.snapshots != nil {
		t.Fatal("已取消请求不应启动采样")
	}
}

func TestEventsFirstFramesAndRequestCancellation(t *testing.T) {
	h := &EventsHandler{}
	blocked, snapshotCanceled := make(chan struct{}), make(chan struct{})
	h.streamsOnce.Do(func() {
		h.systems = newEventSampler(time.Hour, func(context.Context) any { <-blocked; return map[string]int{"cpu": 10} })
		h.snapshots = newEventSampler(time.Hour, func(ctx context.Context) any {
			close(blocked)
			<-ctx.Done()
			close(snapshotCanceled)
			return nil
		})
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &cancelEventResponseWriter{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/events", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() { defer close(done); h.Events(c) }()
	<-blocked
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("取消请求未退出")
	}
	select {
	case <-snapshotCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("重型资源查询未取消")
	}
	if body := writer.Body.String(); !strings.Contains(body, "event: system\ndata: {\"cpu\":10}\n\n") {
		t.Fatalf("重型查询不能阻塞系统首帧：%s", body)
	}
	waitEventSamplerStopped(t, h.systems)
	waitEventSamplerStopped(t, h.snapshots)
}

type cancelEventResponseWriter struct {
	*httptest.ResponseRecorder
	cancel context.CancelFunc
}

func (w *cancelEventResponseWriter) Flush() { w.ResponseRecorder.Flush(); w.cancel() }

func TestWriteEventSamplePreservesSSEContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sample eventSample
		want   string
		ok     bool
	}{
		{"snapshot", eventSample{data: []byte(`{"resources":{},"stacks":[]}`)}, "event: snapshot\ndata: {\"resources\":{},\"stacks\":[]}\n\n", true},
		{"error", eventSample{err: fmt.Errorf("读取 /opt/stacks/example: permission denied")}, "event: error\ndata: \"读取 /opt/stacks/example: permission denied\"\n\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(writer)
			if got := writeEventSample(newSSE(c), tc.name, tc.sample); got != tc.ok {
				t.Fatalf("返回值=%v", got)
			}
			if writer.Body.String() != tc.want {
				t.Fatalf("SSE 契约改变：%q", writer.Body.String())
			}
		})
	}
}
