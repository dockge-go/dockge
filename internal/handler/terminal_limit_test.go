package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTerminalStreamLimitConcurrent(t *testing.T) {
	h := &TerminalHandler{}
	var ready sync.WaitGroup
	var accepted atomic.Int32
	release := make(chan struct{})
	var done sync.WaitGroup
	for range 128 {
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			ok := h.acquireStream()
			if ok {
				accepted.Add(1)
			}
			ready.Done()
			if ok {
				<-release
				h.activeStreams.Add(-1)
			}
		}()
	}
	ready.Wait()
	if n := accepted.Load(); n != maxTerminalStreams {
		t.Errorf("accepted=%d, limit=%d", n, maxTerminalStreams)
	}
	if h.acquireStream() {
		h.activeStreams.Add(-1)
		t.Error("额满仍允许连接")
	}
	close(release)
	done.Wait()
	if n := h.activeStreams.Load(); n != 0 {
		t.Fatalf("结束后遗留 %d 个额度", n)
	}
	if !h.acquireStream() {
		t.Fatal("释放后不能再次连接")
	}
	h.activeStreams.Add(-1)
}

func TestTerminalStreamLimitBeforeRuntimeQuery(t *testing.T) {
	h := &TerminalHandler{}
	h.activeStreams.Store(maxTerminalStreams)
	router := gin.New()
	router.GET("/terminal/:name/:type", h.Events)
	// Repository 留空，确保超额请求不能开始查询或启动 CLI。
	for _, typ := range []string{"container-logs", "compose-logs", "container-exec", "exec"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/terminal/test/"+typ, nil))
		if w.Code != http.StatusTooManyRequests || !strings.Contains(w.Body.String(), "上限 16") {
			t.Fatalf("%s: status=%d body=%s", typ, w.Code, w.Body.String())
		}
		if n := h.activeStreams.Load(); n != maxTerminalStreams {
			t.Fatalf("拒绝连接改变额度: %d", n)
		}
	}
	h.activeStreams.Store(0)
	for range 32 {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/terminal/test/unknown", nil))
		if w.Code != http.StatusBadRequest || h.activeStreams.Load() != 0 {
			t.Fatalf("校验失败未释放额度: status=%d active=%d", w.Code, h.activeStreams.Load())
		}
	}
}
