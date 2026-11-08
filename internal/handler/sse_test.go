package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type deadlineResponseWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *deadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

func TestSSEDeadlineReachesUnderlyingGinWriter(t *testing.T) {
	router := gin.New()
	router.GET("/events", func(c *gin.Context) {
		s := newSSE(c)
		if err := s.event("output", "真实中文\n下一行\r进度"); err != nil {
			t.Error(err)
		}
		s.finish(nil)
	})
	writer := &deadlineResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	before := time.Now()
	router.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/events", nil))
	if writer.deadline.Before(before.Add(9*time.Second)) || writer.deadline.After(time.Now().Add(11*time.Second)) {
		t.Fatalf("Gin 必须将 SSE 写期限传至底层响应：%v", writer.deadline)
	}
	if !strings.HasPrefix(writer.Header().Get("Content-Type"), "text/event-stream") || writer.Header().Get("X-Accel-Buffering") != "no" || !writer.Flushed {
		t.Fatalf("SSE 头及刷新缺失：%v", writer.Header())
	}
	if body := writer.Body.String(); !strings.Contains(body, `data: "真实中文\n下一行\r进度"`) || !strings.Contains(body, "event: done\ndata: true\n\n") {
		t.Fatalf("中文与换行必须保留在完整 JSON 事件中：%q", body)
	}
}

func TestSSEConcurrentFramesDoNotInterleave(t *testing.T) {
	router := gin.New()
	router.GET("/events", func(c *gin.Context) {
		s := newSSE(c)
		var wg sync.WaitGroup
		for i := range 40 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := s.event("output", fmt.Sprintf("中文-%d\n下一行", i)); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
	})
	writer := httptest.NewRecorder()
	router.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/events", nil))
	frames := strings.Split(strings.TrimSuffix(writer.Body.String(), "\n\n"), "\n\n")
	if len(frames) != 40 {
		t.Fatalf("帧数=%d", len(frames))
	}
	for _, frame := range frames {
		if !strings.HasPrefix(frame, "event: output\ndata: ") {
			t.Fatalf("帧交错：%q", frame)
		}
		var data string
		if err := json.Unmarshal([]byte(strings.TrimPrefix(frame, "event: output\ndata: ")), &data); err != nil {
			t.Fatalf("帧 JSON 不完整：%q: %v", frame, err)
		}
	}
}

// Gin 的 Flush 无返回值；回归必须检查只在刷新阶段出现的错误也会到达调用者。
type failedSSEWriter struct {
	*httptest.ResponseRecorder
	deadlineErr error
	flushErr    error
}

func (w *failedSSEWriter) SetWriteDeadline(time.Time) error { return w.deadlineErr }
func (w *failedSSEWriter) FlushError() error                { return w.flushErr }

func TestSSEPropagatesDeadlineAndFlushErrors(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		deadlineErr, flushErr error
	}{
		{name: "deadline", deadlineErr: errors.New("设置连接写期限失败")},
		{name: "flush", flushErr: errors.New("刷新连接超时")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := &failedSSEWriter{ResponseRecorder: httptest.NewRecorder(), deadlineErr: tc.deadlineErr, flushErr: tc.flushErr}
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest(http.MethodGet, "/events", nil)
			err := newSSE(c).event("ping", true)
			want := tc.deadlineErr
			if want == nil {
				want = tc.flushErr
			}
			if !errors.Is(err, want) {
				t.Fatalf("SSE 错误=%v，必须保留底层原因 %v", err, want)
			}
			if tc.deadlineErr != nil && writer.Body.Len() != 0 {
				t.Fatal("期限设置失败后不应继续写入")
			}
		})
	}
}

// 使用真实 TCP 连接而非响应记录器，确认停止读流的客户端不会永久占用请求线程。
func TestSSESlowHTTPClientHitsWriteDeadline(t *testing.T) {
	result := make(chan error, 1)
	router := gin.New()
	router.GET("/events", func(c *gin.Context) {
		s := newSSE(c)
		if err := s.event("ready", true); err != nil {
			result <- err
			return
		}
		// 大于 TCP 收发缓冲区，迫使未消费响应体的连接进入背压。
		result <- s.event("output", strings.Repeat("x", 16*1024*1024))
	})
	server := httptest.NewServer(router)
	defer server.Close()
	defer server.CloseClientConnections()
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Get(server.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("SSE 状态码=%d", response.StatusCode)
	}
	select {
	case err := <-result:
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("停止读取响应体后必须触发真实写入超时，实际错误: %v", err)
		}
	case <-time.After(13 * time.Second):
		t.Fatal("慢客户端超过 SSE 写期限仍占用请求线程")
	}
}
