// Package http 提供基于 Gin 的 HTTP 服务器实现与生命周期管理。
package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	"dockge/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// Server 是嵌入 gin.Engine 的 HTTP 服务器，实现 server.Server 接口。
type Server struct {
	*gin.Engine
	httpSrv *http.Server
	host    string
	port    int
	logger  *log.Logger
	tlsCert string // ssl_cert, empty means HTTP
	tlsKey  string // ssl_key, empty means HTTP
}

// NewServer 按配置构造 HTTP 服务器，由注入容器调用。
func NewServer(i do.Injector) (*Server, error) {
	engine := do.MustInvoke[*gin.Engine](i)
	logger := do.MustInvoke[*log.Logger](i)
	conf := do.MustInvoke[*viper.Viper](i)
	s := &Server{
		Engine:  engine,
		logger:  logger,
		host:    conf.GetString("http.host"),
		port:    conf.GetInt("http.port"),
		tlsCert: conf.GetString("ssl.cert"),
		tlsKey:  conf.GetString("ssl.key"),
	}
	return s, nil
}

// Start 监听并阻塞服务 HTTP 请求（被 app 容器以 goroutine 调起）。
func (s *Server) Start(ctx context.Context) error {
	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.host, s.port),
		Handler: s,
	}

	if s.tlsCert != "" && s.tlsKey != "" {
		cert, loadErr := tls.LoadX509KeyPair(s.tlsCert, s.tlsKey)
		if loadErr != nil {
			s.logger.Fatal().Err(loadErr).Msg("failed to load TLS cert/key")
		}
		s.httpSrv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
		if listenErr := s.httpSrv.ListenAndServeTLS("", ""); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			s.logger.Fatal().Msgf("listen (tls): %s", listenErr)
		}
	} else {
		if listenErr := s.httpSrv.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			s.logger.Fatal().Msgf("listen: %s", listenErr)
		}
	}

	return nil
}

// TLSCert returns whether TLS is enabled.
func (s *Server) TLSCert() string { return s.tlsCert }
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info().Msg("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpSrv.Shutdown(ctx); err != nil {
		s.logger.Fatal().Msgf("Server forced to shutdown: %v", err)
	}

	s.logger.Info().Msg("Server exiting")
	return nil
}
