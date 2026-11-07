package service

import (
	"context"
	"sync"
)

type stackGate struct {
	token chan struct{}
	refs  int
}

var stackLocks = struct {
	sync.Mutex
	gates map[string]*stackGate
}{gates: map[string]*stackGate{}}

// lockStack 让等待中的请求也能取消；最后一个持有者离开后释放名称，删除栈不会遗留锁。
func lockStack(ctx context.Context, name string) (func(), error) {
	stackLocks.Lock()
	gate := stackLocks.gates[name]
	if gate == nil {
		gate = &stackGate{token: make(chan struct{}, 1)}
		gate.token <- struct{}{}
		stackLocks.gates[name] = gate
	}
	gate.refs++
	stackLocks.Unlock()
	releaseRef := func() {
		stackLocks.Lock()
		gate.refs--
		if gate.refs == 0 {
			delete(stackLocks.gates, name)
		}
		stackLocks.Unlock()
	}
	select {
	case <-ctx.Done():
		releaseRef()
		return nil, ctx.Err()
	case <-gate.token:
		if err := ctx.Err(); err != nil {
			gate.token <- struct{}{}
			releaseRef()
			return nil, err
		}
		return func() { gate.token <- struct{}{}; releaseRef() }, nil
	}
}
