// Package app 提供应用生命周期容器：统一启动、信号监听与优雅停止。
package app

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"dockge/pkg/log"
	"dockge/pkg/server"

	"github.com/rs/zerolog"
)

// App 是应用运行时容器：聚合多个 server，负责并行启动、
// 监听退出信号并逐个优雅停止。
type App struct {
	name    string
	servers []server.Server
	logger  *log.Logger
}

// Option 是 App 的函数式可选配置。
type Option func(a *App)

// New builds an App. Without WithLogger, process lifecycle events are discarded.
func New(opts ...Option) *App {
	a := &App{
		logger: &log.Logger{Logger: zerolog.New(io.Discard).With().Timestamp().Logger()},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// WithServer 注册需要随应用启动/停止的 server。
func WithServer(servers ...server.Server) Option {
	return func(a *App) {
		a.servers = servers
	}
}

// WithName 设置应用名。
func WithName(name string) Option {
	return func(a *App) {
		a.name = name
	}
}

// WithLogger 设置应用日志器。
func WithLogger(logger *log.Logger) Option {
	return func(a *App) {
		a.logger = logger
	}
}

// Run 启动全部 server 并阻塞，直至收到 SIGINT/SIGTERM 或 ctx 取消，
// 随后依次调用各 server 的 Stop 完成优雅退出。
func (a *App) Run(ctx context.Context) error {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	for _, srv := range a.servers {
		go func(srv server.Server) {
			if err := srv.Start(ctx); err != nil {
				a.logger.Error().Err(err).Msg("server start")
			}
		}(srv)
	}

	select {
	case sig := <-signals:
		a.logger.Info().Str("signal", sig.String()).Msg("received termination signal")
	case <-ctx.Done():
		a.logger.Info().Msg("context canceled")
	}

	for _, srv := range a.servers {
		if err := srv.Stop(ctx); err != nil {
			a.logger.Error().Err(err).Msg("server stop")
		}
	}

	return nil
}
