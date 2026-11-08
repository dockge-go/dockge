package handler

import (
	"context"
	"testing"
	"time"
)

func TestSampleEventsDoesNotReadCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := make(chan int, 1)
	sampleEvents(ctx, time.Second, out, func(context.Context) int {
		t.Fatal("已取消请求不应启动下一轮采样")
		return 0
	})
	if len(out) != 0 {
		t.Fatal("已取消请求不应推送数据")
	}
}

func TestSampleEventsCancellationReleasesBlockedPublisher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan int)
	sampled := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		sampleEvents(ctx, time.Second, out, func(context.Context) int {
			close(sampled)
			return 1
		})
	}()
	<-sampled
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("客户端不消费时取消仍应释放采样线程")
	}
}
