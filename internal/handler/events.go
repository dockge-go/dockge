package handler

import (
	"context"
	v1 "dockge/api/v1"
	"dockge/internal/repository"
	"dockge/internal/service"
	"dockge/internal/version"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"time"
)

// EventsHandler 独立采样主机与资源，重型运行时查询不会阻塞 CPU/内存推送。
type EventsHandler struct {
	Resources  *service.ResourceService `do:""`
	Stacks     service.StackService     `do:""`
	Auth       service.AuthService      `do:""`
	Repository *repository.Repository   `do:""`
}

// NewEventsHandler 注入真实采样服务。
func NewEventsHandler(i do.Injector) (*EventsHandler, error) {
	return do.MustInvokeStruct[*EventsHandler](i), nil
}

// Events 以一个认证 SSE 连接推送完整资源快照及轻量系统采样。
func (h *EventsHandler) Events(c *gin.Context) {
	ctx, cancel := streamContext(c, h.Auth)
	defer cancel()
	s := newSSE(c)
	snapshots := make(chan any, 1)
	systems := make(chan *v1.HostSystem, 1)
	go sampleEvents(ctx, 2*time.Second, systems, func(ctx context.Context) *v1.HostSystem { return h.Resources.System(ctx) })
	go sampleEvents(ctx, 5*time.Second, snapshots, func(ctx context.Context) any {
		bounded, stop := context.WithTimeout(ctx, 20*time.Second)
		defer stop()
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
		return gin.H{"resources": resources, "stacks": stacks, "health": gin.H{"version": version.Version, "runtime": h.Repository.Preflight(bounded)}}
	})
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-systems:
			if s.event("system", data) != nil {
				return
			}
		case data := <-snapshots:
			if s.event("snapshot", data) != nil {
				return
			}
		case <-heartbeat.C:
			if s.event("ping", time.Now().UnixMilli()) != nil {
				return
			}
		}
	}
}
func sampleEvents[T any](ctx context.Context, period time.Duration, out chan<- T, read func(context.Context) T) {
	for {
		data := read(ctx)
		select {
		case <-ctx.Done():
			return
		case out <- data:
		}
		timer := time.NewTimer(period)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
