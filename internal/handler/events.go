package handler

import (
	"context"
	"errors"
	"sync"
	"time"

	v1 "dockge/api/v1"
	"dockge/internal/service"
	"dockge/internal/version"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// EventsHandler 在活跃连接间共享主机与资源采样，避免多窗口重复查询运行时。
type EventsHandler struct {
	Resources   *service.ResourceService `do:""`
	Stacks      service.StackService     `do:""`
	Auth        service.AuthService      `do:""`
	streamsOnce sync.Once
	systems     *eventSampler
	snapshots   *eventSampler
}

// NewEventsHandler 注入真实采样服务。
func NewEventsHandler(i do.Injector) (*EventsHandler, error) {
	return do.MustInvokeStruct[*EventsHandler](i), nil
}

// Events 以一个认证 SSE 连接推送完整资源快照及轻量系统采样。
func (h *EventsHandler) Events(c *gin.Context) {
	ctx, cancel := streamContext(c, h.Auth)
	defer cancel()
	if ctx.Err() != nil {
		return
	}
	s := newSSE(c)
	h.initStreams()
	systems, stopSystems := h.systems.subscribe()
	defer stopSystems()
	snapshots, stopSnapshots := h.snapshots.subscribe()
	defer stopSnapshots()
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-systems:
			if !writeEventSample(s, "system", data) {
				return
			}
		case data := <-snapshots:
			if !writeEventSample(s, "snapshot", data) {
				return
			}
		case <-heartbeat.C:
			if s.event("ping", time.Now().UnixMilli()) != nil {
				return
			}
		}
	}
}

func (h *EventsHandler) initStreams() {
	h.streamsOnce.Do(func() {
		h.systems = newEventSampler(2*time.Second, func(ctx context.Context, _ uint64) any { return h.Resources.System(ctx) })
		h.snapshots = newEventSampler(5*time.Second, h.snapshot)
	})
}

// Refresh 仅触发采样，真实数据仍通过现有 SSE 返回；不重建连接或回放历史。
func (h *EventsHandler) Refresh(c *gin.Context) {
	if c.Request.Context().Err() != nil {
		return
	}
	h.initStreams()
	revision := h.snapshots.refresh()
	h.systems.refresh()
	c.Header("Cache-Control", "no-store")
	v1.HandleSuccess(c, gin.H{"revision": revision})
}

// snapshot 聚合真实资源与轻量栈配置，不启动逐栈容器详情查询。
func (h *EventsHandler) snapshot(ctx context.Context, revision uint64) any {
	bounded, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()
	health := h.Resources.RuntimeStatus(bounded)
	resources := h.Resources.Snapshot(bounded)
	stacks, err := h.Stacks.Snapshot(bounded)
	if err != nil {
		resources.Errors["stacks"] = err.Error()
		stacks = []v1.StackConfigData{}
	}
	return gin.H{"revision": revision, "resources": resources, "stacks": stacks, "health": gin.H{"version": version.Version, "runtime": health}}
}

func writeEventSample(s *sseWriter, name string, sample eventSample) bool {
	if sample.err != nil {
		_ = s.event("error", sample.err.Error())
		return false
	}
	return s.eventJSON(name, sample.data) == nil
}

// Shutdown 等待共享查询回收，避免 HTTP 退出后后台任务继续访问已关闭的依赖。
func (h *EventsHandler) Shutdown(ctx context.Context) error {
	h.initStreams()
	return errors.Join(h.systems.shutdown(ctx), h.snapshots.shutdown(ctx))
}
