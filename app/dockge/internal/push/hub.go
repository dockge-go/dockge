package push

import (
	"sync"

	"github.com/samber/do/v2"
)

// StatusHub 是容器状态的 SSE 广播中心：
// ContainerStatusServer 产出状态帧，所有订阅连接各持一个通道接收。
// Broadcast 非阻塞发送（慢客户端丢弃本次，以下一次快照为准）。
type StatusHub struct {
	mu          sync.RWMutex
	clients     map[chan []byte]struct{}
	lastPayload []byte
}

// NewStatusHub 构造广播中心，由注入容器调用。
func NewStatusHub(i do.Injector) (*StatusHub, error) {
	return &StatusHub{clients: make(map[chan []byte]struct{})}, nil
}

// Subscribe 注册一个客户端通道，并返回最近一次状态帧供立即下发。
func (h *StatusHub) Subscribe() (chan []byte, []byte) {
	ch := make(chan []byte, 4)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	last := h.lastPayload
	h.mu.Unlock()
	return ch, last
}

// Unsubscribe 注销客户端通道。
func (h *StatusHub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

// Broadcast 向全部客户端推送状态帧并记录为最新帧。
func (h *StatusHub) Broadcast(payload []byte) {
	h.mu.Lock()
	h.lastPayload = payload
	clients := make([]chan []byte, 0, len(h.clients))
	for ch := range h.clients {
		clients = append(clients, ch)
	}
	h.mu.Unlock()

	for _, ch := range clients {
		select {
		case ch <- payload:
		default:
		}
	}
}
