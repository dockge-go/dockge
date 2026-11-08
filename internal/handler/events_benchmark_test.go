package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	v1 "dockge/api/v1"
	"github.com/gin-gonic/gin"
)

// 使用固定测试夹具隔离快照编码成本；不访问业务容器，也不推断线上 CPU 占用。
func benchmarkEventSnapshot() any {
	resources := &v1.ResourceSnapshot{Errors: map[string]string{}, Runtime: "docker"}
	for i := range 64 {
		resources.Containers = append(resources.Containers, v1.ResourceContainer{
			ID: fmt.Sprintf("%064d", i), Names: []string{fmt.Sprintf("/container-%d", i)},
			Env:   []string{"CONFIG=" + strings.Repeat("x", 1024)},
			Stats: &v1.ResourceStats{CPUPct: 1, MemMB: 128, NetIO: "1MB / 2MB"},
		})
	}
	return map[string]any{"resources": resources, "stacks": []map[string]string{{
		"name": "example", "yaml": strings.Repeat("# configuration\n", 4096), "env": "KEY=value",
	}}}
}

var benchmarkEventBytes []byte

func BenchmarkEventsEncoding(b *testing.B) {
	payload := benchmarkEventSnapshot()
	for _, clients := range []int{1, 8, 32} {
		b.Run(fmt.Sprintf("per-connection/%d", clients), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for range clients {
					data, err := json.Marshal(payload)
					if err != nil {
						b.Fatal(err)
					}
					benchmarkEventBytes = data
				}
			}
		})
		b.Run(fmt.Sprintf("shared/%d", clients), func(b *testing.B) {
			queues := make([]chan eventSample, clients)
			for i := range queues {
				queues[i] = make(chan eventSample, 1)
			}
			b.ReportAllocs()
			for b.Loop() {
				data, err := json.Marshal(payload)
				if err != nil {
					b.Fatal(err)
				}
				for _, out := range queues {
					sendLatestEvent(out, eventSample{data: data})
				}
				benchmarkEventBytes = data
			}
		})
	}
}

// 计入真实 SSE 写帧路径，但排除网络速度和运行时子进程耗时。
type discardEventsWriter struct{ header http.Header }

func (w *discardEventsWriter) Header() http.Header       { return w.header }
func (*discardEventsWriter) Write(p []byte) (int, error) { return len(p), nil }
func (*discardEventsWriter) WriteHeader(int)             {}
func (*discardEventsWriter) Flush()                      {}

func BenchmarkEventsDelivery(b *testing.B) {
	payload := benchmarkEventSnapshot()
	for _, clients := range []int{1, 8, 32} {
		for _, shared := range []bool{false, true} {
			name := "per-connection"
			if shared {
				name = "shared"
			}
			b.Run(fmt.Sprintf("%s/%d", name, clients), func(b *testing.B) {
				writers := make([]*sseWriter, clients)
				for i := range writers {
					c, _ := gin.CreateTestContext(&discardEventsWriter{header: make(http.Header)})
					writers[i] = newSSE(c)
				}
				b.ReportAllocs()
				for b.Loop() {
					if shared {
						data, err := json.Marshal(payload)
						if err != nil {
							b.Fatal(err)
						}
						for _, writer := range writers {
							if err := writer.eventJSON("snapshot", data); err != nil {
								b.Fatal(err)
							}
						}
					} else {
						for _, writer := range writers {
							if err := benchmarkLegacyEvent(writer, "snapshot", payload); err != nil {
								b.Fatal(err)
							}
						}
					}
				}
			})
		}
	}
}

// 固定改动前的编码/写帧路径，使后续基准仍能与原实现比较。
func benchmarkLegacyEvent(s *sseWriter, name string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rc.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := fmt.Fprintf(s.c.Writer, "event: %s\ndata: %s\n\n", name, data); err != nil {
		return err
	}
	return s.rc.Flush()
}
