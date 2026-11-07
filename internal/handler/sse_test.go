package handler

import (
	"encoding/json"
	"fmt"
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
