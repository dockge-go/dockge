package handler

import (
	"context"
	"sync"
	"time"

	"dockge/internal/repository"
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
	Repository  *repository.Repository   `do:""`
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
	h.streamsOnce.Do(func() {
		h.systems = newEventSampler(2*time.Second, func(ctx context.Context) any { return h.Resources.System(ctx) })
		h.snapshots = newEventSampler(5*time.Second, h.snapshot)
	})
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

// snapshot 保持资源查询与归属校验不变，共享采样只消除连接间重复工作。
func (h *EventsHandler) snapshot(ctx context.Context) any {
	bounded, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()
	health := h.Repository.Preflight(bounded)
	resources := h.Resources.Snapshot(bounded)
	stacks := make([]any, 0)
	list, err := h.Stacks.List(bounded, "")
	if err != nil {
		resources.Errors["stacks"] = err.Error()
	} else {
		for _, summary := range list.List {
			d, e := h.Stacks.Get(bounded, summary.Name)
			if e != nil {
				stacks = append(stacks, gin.H{"name": summary.Name, "status": summary.Status, "managed": summary.Managed, "configFiles": summary.ConfigFiles, "loadError": e.Error()})
			} else {
				stacks = append(stacks, d)
			}
		}
	}
	return gin.H{"resources": resources, "stacks": stacks, "health": gin.H{"version": version.Version, "runtime": health}}
}

func writeEventSample(s *sseWriter, name string, sample eventSample) bool {
	if sample.err != nil {
		_ = s.event("error", sample.err.Error())
		return false
	}
	return s.eventJSON(name, sample.data) == nil
}
