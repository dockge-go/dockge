package repository

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func preflightRuntime(t *testing.T, body string) *Repository {
	t.Helper()
	dir := t.TempDir()
	cli := filepath.Join(dir, "runtime")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	return &Repository{stacksDir: dir, runtime: DetectRuntime(cli, "")}
}

func TestPreflightCanceledRequestDoesNotPoisonCache(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "取消", true: "到期"}[expired], func(t *testing.T) {
			r := preflightRuntime(t, "exit 0\n")
			ctx, cancel := context.WithCancel(context.Background())
			if expired {
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			}
			cancel()
			first := r.Preflight(ctx)
			if first.Ready || !strings.Contains(first.Error, ctx.Err().Error()) {
				t.Fatalf("取消请求: %+v", first)
			}
			if got := r.Preflight(context.Background()); !got.Ready {
				t.Fatalf("正常请求被取消缓存污染: %+v", got)
			}
		})
	}
}

func TestPreflightWaitingRequestCanCancel(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	r := preflightRuntime(t, "echo started > '"+marker+"'\nsleep 0.4\nexit 0\n")
	// 等待脚本启动，避免请求是否在途取决于调度顺序。
	done := make(chan RuntimeStatus, 1)
	go func() { done <- r.Preflight(context.Background()) }()
	waitForProbeMarker(t, marker)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	got := r.Preflight(ctx)
	if time.Since(start) > 200*time.Millisecond || !strings.Contains(got.Error, context.DeadlineExceeded.Error()) {
		t.Errorf("等待自检的请求不能及时取消: elapsed=%s status=%+v", time.Since(start), got)
	}
	if first := <-done; !first.Ready {
		t.Fatalf("首个自检异常: %+v", first)
	}
}

func TestPreflightConcurrentRequestsShareOneProbe(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "calls")
	r := preflightRuntime(t, "echo info >> '"+marker+"'\nsleep 0.1\nexit 0\n")
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := r.Preflight(context.Background()); !got.Ready {
				t.Errorf("自检异常: %+v", got)
			}
		}()
	}
	wg.Wait()
	calls, err := os.ReadFile(marker)
	if err != nil || strings.Count(string(calls), "info") != 1 {
		t.Fatalf("并发自检重复启动: %q %v", calls, err)
	}
}

func TestPreflightCallerCancellationDoesNotCancelSharedProbe(t *testing.T) {
	r := preflightRuntime(t, "sleep 0.2\nexit 0\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	got := r.Preflight(ctx)
	if !strings.Contains(got.Error, context.DeadlineExceeded.Error()) {
		t.Fatalf("调用方到期未保留原因: %+v", got)
	}
	if next := r.Preflight(context.Background()); !next.Ready {
		t.Fatalf("共享自检被调用方取消: %+v", next)
	}
}

func TestPreflightRealFailureKeepsRuntimeDetails(t *testing.T) {
	r := preflightRuntime(t, "echo 'socket permission denied' >&2\nexit 7\n")
	got := r.Preflight(context.Background())
	if got.Ready || !strings.Contains(got.Error, "socket permission denied") || !strings.Contains(got.Error, "exit status 7") {
		t.Fatalf("自检错误丢失真实细节: %+v", got)
	}
}

func waitForProbeMarker(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("等待测试子进程启动超时: %s", path)
}
