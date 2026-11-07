package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"dockge/internal/service"
	"dockge/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// sseWriter 统一帧格式与写入期限，慢客户端不能无限占住子进程。
type sseWriter struct {
	c  *gin.Context
	mu sync.Mutex
}

func newSSE(c *gin.Context) *sseWriter {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	return &sseWriter{c: c}
}
func (s *sseWriter) event(name string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rc := http.NewResponseController(s.c.Writer)
	_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err = fmt.Fprintf(s.c.Writer, "event: %s\ndata: %s\n\n", name, b); err != nil {
		return err
	}
	return rc.Flush()
}
func (s *sseWriter) Write(p []byte) (int, error) {
	if err := s.event("output", string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}
func (s *sseWriter) finish(err error) {
	if err != nil {
		_ = s.event("error", err.Error())
		return
	}
	_ = s.event("done", true)
}

// streamContext 在长连接期间复核密码绑定与 JWT 到期，注销不会遗留终端进程。
func streamContext(c *gin.Context, auth service.AuthService) (context.Context, context.CancelFunc) {
	parent := c.Request.Context()
	value, _ := c.Get("claims")
	claims, _ := value.(*jwt.Claims)
	var ctx context.Context
	var cancel context.CancelFunc
	if claims != nil && claims.ExpiresAt != nil {
		ctx, cancel = context.WithDeadline(parent, claims.ExpiresAt.Time)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	if auth != nil && claims != nil {
		go func() {
			tick := time.NewTicker(5 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					if auth.CheckSession(ctx, claims.UserId, claims.H) != nil {
						cancel()
						return
					}
				}
			}
		}()
	}
	return ctx, cancel
}

func (s *sseWriter) heartbeat(ctx context.Context, cancel context.CancelFunc) func() {
	ctx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if s.event("ping", true) != nil {
					cancel()
					return
				}
			}
		}
	}()
	return func() { stop(); <-done }
}
