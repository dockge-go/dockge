package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// data 在发布后不再修改；所有连接复用同一份 JSON，而非保留事件历史。
type eventSample struct {
	data []byte
	err  error
}

type eventSampler struct {
	mu       sync.Mutex
	read     func(context.Context) any
	period   time.Duration
	clients  map[chan eventSample]uint64
	revision uint64
	running  bool
	cancel   context.CancelFunc
	wake     chan struct{}
}

func newEventSampler(period time.Duration, read func(context.Context) any) *eventSampler {
	return &eventSampler{read: read, period: period, clients: make(map[chan eventSample]uint64), wake: make(chan struct{}, 1)}
}

func (s *eventSampler) subscribe() (<-chan eventSample, func()) {
	out := make(chan eventSample, 1)
	s.mu.Lock()
	s.revision++
	s.clients[out] = s.revision
	if !s.running {
		s.running = true
		go s.run()
	} else {
		s.notify()
	}
	s.mu.Unlock()
	return out, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, exists := s.clients[out]; !exists {
			return
		}
		delete(s.clients, out)
		// 释放待发快照；不关闭通道，避免消费端把零值误当成新数据。
		select {
		case <-out:
		default:
		}
		if len(s.clients) == 0 {
			if s.cancel != nil {
				s.cancel()
			}
			s.notify()
		}
	}
}

// notify 在 mu 内调用；唤醒最多排队一次，重连不会堆积采样任务。
func (s *eventSampler) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *eventSampler) run() {
	for {
		s.mu.Lock()
		if len(s.clients) == 0 {
			s.running = false
			s.mu.Unlock()
			return
		}
		select {
		case <-s.wake:
		default:
		}
		revision := s.revision
		// 不继承某个请求的取消：一个窗口退出不能中断其余窗口。
		// 最后订阅者退出仍会取消当前查询，新订阅等待旧查询回收后再启动。
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		s.mu.Unlock()

		data := s.read(ctx)
		sample := eventSample{}
		if ctx.Err() == nil {
			sample.data, sample.err = json.Marshal(data)
			if sample.err != nil {
				sample.err = fmt.Errorf("SSE 快照 JSON 编码失败: %w", sample.err)
			}
		}

		s.mu.Lock()
		canceled := ctx.Err() != nil
		cancel()
		s.cancel = nil
		if !canceled {
			for out, joined := range s.clients {
				// 操作完成后重连必须收到重连之后开始的查询，不能把旧在途结果当作刷新成功。
				if joined > revision {
					continue
				}
				sendLatestEvent(out, sample)
			}
		}
		s.mu.Unlock()
		if canceled {
			continue
		}

		timer := time.NewTimer(s.period)
		select {
		case <-s.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

// 只有采样线程写入；慢连接只留一个最新值，绝不阻塞其他订阅或累积历史帧。
func sendLatestEvent(out chan eventSample, sample eventSample) {
	select {
	case out <- sample:
		return
	default:
	}
	select {
	case <-out:
	default:
	}
	out <- sample
}
