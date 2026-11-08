// Package server 服务器装配：gin 引擎、中间件、路由与优雅启停。
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"
	"time"

	"dockge/internal/handler"
	"dockge/internal/middleware"
	"dockge/internal/service"
	"dockge/pkg/jwt"
	"dockge/pkg/log"
	"dockge/web"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// Package 注册本层全部服务。
var Package = do.Package(
	do.Lazy(NewHTTPServer),
)

// HTTPServer 包装 gin 引擎与 *http.Server；依赖经 do:"" 标签由容器在构造期注入，
// 实现 ShutdownerWithContextAndError 参与容器优雅关闭。
type HTTPServer struct {
	Config             *viper.Viper                `do:""`
	Logger             *log.Logger                 `do:""`
	JWT                *jwt.JWT                    `do:""`
	AuthService        service.AuthService         `do:""`
	OIDCHandler        *handler.OIDCHandler        `do:""`
	AuthHandler        *handler.AuthHandler        `do:""`
	StackHandler       *handler.StackHandler       `do:""`
	ResourceHandler    *handler.ResourceHandler    `do:""`
	ImageHandler       *handler.ImageHandler       `do:""`
	DockerHandler      *handler.DockerHandler      `do:""`
	SettingsHandler    *handler.SettingsHandler    `do:""`
	ComposerizeHandler *handler.ComposerizeHandler `do:""`
	TerminalHandler    *handler.TerminalHandler    `do:""`

	EventsHandler *handler.EventsHandler `do:""`

	engine  *gin.Engine
	tlsCert string // ssl_cert, 空表示 HTTP
	tlsKey  string // ssl_key, 空表示 HTTP
	*nethttp.Server
}

// NewHTTPServer 装配静态资源、中间件、路由并构造 HTTP 服务器。
func NewHTTPServer(i do.Injector) (*HTTPServer, error) {
	s := do.MustInvokeStruct[*HTTPServer](i)

	if s.Config.GetString("env") == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	s.tlsCert = s.Config.GetString("ssl.cert")
	s.tlsKey = s.Config.GetString("ssl.key")

	engine := gin.New()
	// 认证/业务层收到 Gin context 时也能感知请求断开及截止时间。
	engine.ContextWithFallback = true
	// 未配置可信代理：转发头不能由直接客户端伪造来绕过登录限流。
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("设置 HTTP 可信代理：%w", err)
	}
	engine.Use(middleware.Recovery(s.Logger))

	embedFolder, err := static.EmbedFolder(web.Assets(), "dist")
	if err != nil {
		return nil, err
	}
	// SPA shell（index.html 及无扩展名路径）禁止缓存；/assets 下的文件
	// 文件名自带内容 hash，可长期缓存。否则发版后浏览器继续跑旧 bundle。
	engine.Use(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "no-cache")
		}
		c.Next()
	})
	engine.Use(static.Serve("/", embedFolder))
	engine.NoRoute(func(c *gin.Context) {
		// API 路径未命中路由时返回 404 JSON，避免把 SPA index.html 当响应体
		if strings.HasPrefix(c.Request.URL.Path, "/v1/") {
			c.AbortWithStatusJSON(nethttp.StatusNotFound, gin.H{
				"code":    nethttp.StatusNotFound,
				"message": "接口不存在: " + c.Request.Method + " " + c.Request.URL.Path,
			})
			return
		}
		indexPageData, err := web.Assets().ReadFile("dist/index.html")
		if err != nil {
			c.String(nethttp.StatusNotFound, "404 page not found")
			return
		}
		// SPA shell 不缓存：assets 文件名自带 hash 可长缓存，但 index.html
		// 必须每次回源，否则发版后浏览器会继续跑旧 bundle
		c.Header("Cache-Control", "no-cache")
		c.Data(nethttp.StatusOK, "text/html; charset=utf-8", indexPageData)
	})

	engine.Use(middleware.CORS(), middleware.RequestLogger(s.Logger))

	v1Group := engine.Group("/v1")
	v1Group.Use(middleware.SetupRequired(s.AuthService))
	{
		noAuthRouter := v1Group.Group("/")
		{
			noAuthRouter.GET("/auth/config", s.OIDCHandler.Config)
			noAuthRouter.GET("/auth/oidc/login", s.OIDCHandler.Login)
			noAuthRouter.GET("/auth/oidc/callback", s.OIDCHandler.Callback)
			noAuthRouter.POST("/auth/oidc/session", s.OIDCHandler.Session)
			noAuthRouter.POST("/login", s.AuthHandler.Login)
			noAuthRouter.POST("/setup", s.AuthHandler.Setup)
			noAuthRouter.GET("/setup/need", s.AuthHandler.NeedSetup)
			noAuthRouter.GET("/health", s.DockerHandler.Health)
			noAuthRouter.GET("/robots.txt", func(c *gin.Context) {
				c.String(200, "User-agent: *\nDisallow: /")
			})
			noAuthRouter.POST("/auto-login", s.AuthHandler.AutoLogin)
		}

		strictAuthRouter := v1Group.Group("/")
		strictAuthRouter.Use(middleware.StrictAuth(s.JWT, s.Logger, s.AuthService))
		{
			strictAuthRouter.GET("/me", s.AuthHandler.Me)
			strictAuthRouter.PUT("/me/password", s.AuthHandler.ChangePassword)
			strictAuthRouter.GET("/me/disableauth", s.AuthHandler.GetDisableAuth)
			strictAuthRouter.POST("/me/disableauth", s.AuthHandler.ToggleDisableAuth)

			strictAuthRouter.GET("/events", s.EventsHandler.Events)
			strictAuthRouter.GET("/resources", s.ResourceHandler.Snapshot)
			strictAuthRouter.GET("/system", s.ResourceHandler.System)
			strictAuthRouter.POST("/resources/:kind", s.ResourceHandler.Create)
			strictAuthRouter.POST("/resources/:kind/:id/:action", s.ResourceHandler.Action)
			strictAuthRouter.GET("/resources/images/:id/history", s.ResourceHandler.History)
			strictAuthRouter.GET("/stacks/:name/delete-plan", s.StackHandler.DeletePlan)
			strictAuthRouter.GET("/stacks/networks", s.StackHandler.Networks)
			strictAuthRouter.GET("/stacks", s.StackHandler.List)
			strictAuthRouter.POST("/stacks", s.StackHandler.Create)
			strictAuthRouter.GET("/stacks/:name", s.StackHandler.Get)
			strictAuthRouter.PUT("/stacks/:name", s.StackHandler.Update)
			strictAuthRouter.DELETE("/stacks/:name", s.StackHandler.Delete)
			strictAuthRouter.POST("/stacks/:name/:op", s.StackHandler.Op)
			strictAuthRouter.POST("/stacks/:name/services/:service/:op", s.StackHandler.ServiceOp)
			strictAuthRouter.GET("/stacks/:name/stats", s.StackHandler.Stats)

			strictAuthRouter.GET("/images", s.ImageHandler.List)
			strictAuthRouter.POST("/images/pull", s.ImageHandler.Pull)
			strictAuthRouter.POST("/images/delete", s.ImageHandler.Delete)

			strictAuthRouter.GET("/settings/globalenv", s.SettingsHandler.GetGlobalEnv)
			strictAuthRouter.PUT("/settings/globalenv", s.SettingsHandler.SetGlobalEnv)
			strictAuthRouter.GET("/settings/primaryhostname", s.SettingsHandler.GetPrimaryHostname)
			strictAuthRouter.PUT("/settings/primaryhostname", s.SettingsHandler.SetPrimaryHostname)

			strictAuthRouter.POST("/composerize", s.ComposerizeHandler.Convert)

			strictAuthRouter.GET("/terminal/:name/:type", s.TerminalHandler.Events)
			strictAuthRouter.POST("/terminal/sessions/:id", s.TerminalHandler.Input)
		}
	}

	s.engine = engine
	s.Server = &nethttp.Server{
		Addr:    fmt.Sprintf("%s:%d", s.Config.GetString("http.host"), s.Config.GetInt("http.port")),
		Handler: engine,
		// 防御慢速连接与空闲连接堆积；不设 Read/Write 超时——
		// 栈操作是长时流式响应、终端是 SSE，均需长连接。
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return s, nil
}

// Start 阻塞监听（TLS 配置存在时启用 HTTPS）；返回 http.ErrServerClosed 表示正常关闭。
func (s *HTTPServer) Start() error {
	if s.tlsCert != "" && s.tlsKey != "" {
		cert, err := tls.LoadX509KeyPair(s.tlsCert, s.tlsKey)
		if err != nil {
			return fmt.Errorf("load TLS cert/key: %w", err)
		}
		s.Server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
		if err := s.Server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			return err
		}
		return nil
	}
	if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown 实现 do.ShutdownerWithContextAndError，容器关闭时被调用。
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	if err := s.Server.Shutdown(ctx); err != nil {
		// Shutdown 超时并不会关闭活动连接；先终止请求再允许容器回收依赖。
		if closeErr := s.Server.Close(); closeErr != nil {
			return errors.Join(fmt.Errorf("HTTP 优雅关闭：%w", err), fmt.Errorf("HTTP 强制关闭连接：%w", closeErr))
		}
		return fmt.Errorf("HTTP 优雅关闭超时或取消，已强制关闭连接：%w", err)
	}
	return nil
}

// TLSEnabled 返回是否启用 HTTPS。
func (s *HTTPServer) TLSEnabled() bool { return s.tlsCert != "" && s.tlsKey != "" }
