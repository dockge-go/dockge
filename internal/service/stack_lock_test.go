package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStackLockCancellationAndRelease(t *testing.T) {
	name := t.Name()
	unlock, err := lockStack(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if release, err := lockStack(ctx, name); !errors.Is(err, context.DeadlineExceeded) || release != nil {
		t.Fatalf("等待取消: %v", err)
	}
	// 不同栈不应被全局锁阻塞。
	other, err := lockStack(context.Background(), name+"other")
	if err != nil {
		t.Fatal(err)
	}
	other()
	done := make(chan struct{})
	go func() {
		next, err := lockStack(context.Background(), name)
		if err == nil {
			next()
		}
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("同栈锁未互斥")
	case <-time.After(20 * time.Millisecond):
	}
	unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("锁未释放")
	}
	stackLocks.Lock()
	_, exists := stackLocks.gates[name]
	stackLocks.Unlock()
	if exists {
		t.Fatal("删除后锁条目遗留")
	}
}
