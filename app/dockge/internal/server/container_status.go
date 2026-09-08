package server

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"sync"
	"time"

	v1 "dockge/app/dockge/api/v1"
	"dockge/app/dockge/internal/push"
	"dockge/app/dockge/internal/repository"
	"dockge/pkg/log"

	"github.com/samber/do/v2"
)

// ContainerStatusServer 监听 Docker 事件并广播容器状态帧。
type ContainerStatusServer struct {
	hub    *push.StatusHub
	repo   *repository.Repository
	logger *log.Logger

	cancel     context.CancelFunc
	cancelMu   sync.Mutex
	cmd        *exec.Cmd
	broadcastC chan struct{}
}

// NewContainerStatusServer 构造容器状态推送服务。
func NewContainerStatusServer(i do.Injector) (*ContainerStatusServer, error) {
	return &ContainerStatusServer{
		hub:        do.MustInvoke[*push.StatusHub](i),
		repo:       do.MustInvoke[*repository.Repository](i),
		logger:     do.MustInvoke[*log.Logger](i),
		broadcastC: make(chan struct{}, 1),
	}, nil
}

// Start 启动 Docker 事件监听、状态广播与兜底采样。
func (s *ContainerStatusServer) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)
	s.scheduleBroadcast()
	go s.watchDockerEvents(ctx)
	go s.tickerLoop(ctx)
	go s.broadcastLoop(ctx)
	<-ctx.Done()
	return nil
}

// Stop 停止事件监听与状态广播。
func (s *ContainerStatusServer) Stop(context.Context) error {
	s.cancelMu.Lock()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	cancel := s.cancel
	s.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (s *ContainerStatusServer) scheduleBroadcast() {
	select {
	case s.broadcastC <- struct{}{}:
	default:
	}
}

func (s *ContainerStatusServer) tickerLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scheduleBroadcast()
		}
	}
}

func (s *ContainerStatusServer) broadcastLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.broadcastC:
			timer := time.NewTimer(500 * time.Millisecond)
		drain:
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-s.broadcastC:
				case <-timer.C:
					break drain
				}
			}
			s.broadcast(ctx)
		}
	}
}

func (s *ContainerStatusServer) watchDockerEvents(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "docker", "events", "--filter", "type=container", "--format", "{{json .}}")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.logger.Error().Err(err).Msg("docker events stdout pipe")
		return
	}
	s.cancelMu.Lock()
	s.cmd = cmd
	s.cancelMu.Unlock()
	if err := cmd.Start(); err != nil {
		s.logger.Error().Err(err).Msg("start docker container events")
		return
	}
	defer func() { _ = cmd.Wait() }()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		s.scheduleBroadcast()
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		s.logger.Error().Err(err).Msg("docker container events scanner")
	}
}

func (s *ContainerStatusServer) broadcast(ctx context.Context) {
	containers, err := s.repo.DockerContainers(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("collect container status")
		return
	}
	frame := v1.ContainerStatusFrame{Containers: make([]v1.ContainerStatusData, 0, len(containers))}
	for _, container := range containers {
		frame.Containers = append(frame.Containers, v1.ContainerStatusData{
			ID: container.ID, State: container.State, Status: container.Status,
		})
	}
	payload, err := json.Marshal(frame)
	if err != nil {
		s.logger.Error().Err(err).Msg("marshal container status")
		return
	}
	s.hub.Broadcast(payload)
}
