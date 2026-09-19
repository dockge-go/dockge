package server

import (
	nethttp "net/http"
	"strings"

	"dockge/internal/handler"
	"dockge/internal/middleware"
	"dockge/internal/service"
	"dockge/pkg/jwt"
	"dockge/pkg/log"
	httpx "dockge/pkg/server/http"
	"dockge/web"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// NewHTTPServer 装配静态资源、路由、中间件并返回 HTTP server。
func NewHTTPServer(i do.Injector) (*httpx.Server, error) {
	logger := do.MustInvoke[*log.Logger](i)
	j := do.MustInvoke[*jwt.JWT](i)
	authHandler := do.MustInvoke[*handler.AuthHandler](i)
	authService := do.MustInvoke[service.AuthService](i)
	stackHandler := do.MustInvoke[*handler.StackHandler](i)
	dockerHandler := do.MustInvoke[*handler.DockerHandler](i)
	settingsHandler := do.MustInvoke[*handler.SettingsHandler](i)
	composerizeHandler := do.MustInvoke[*handler.ComposerizeHandler](i)
	terminalHandler := do.MustInvoke[*handler.TerminalHandler](i)

	if do.MustInvoke[*viper.Viper](i).GetString("env") == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	s, err := httpx.NewServer(i)
	if err != nil {
		return nil, err
	}

	embedFolder, err := static.EmbedFolder(web.Assets(), "dist")
	if err != nil {
		return nil, err
	}
	// SPA shell（index.html 及无扩展名路径）禁止缓存；/assets 下的文件
	// 文件名自带内容 hash，可长期缓存。否则发版后浏览器继续跑旧 bundle。
	s.Use(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "no-cache")
		}
		c.Next()
	})
	s.Use(static.Serve("/", embedFolder))
	s.NoRoute(func(c *gin.Context) {
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

	s.Use(middleware.CORSMiddleware(), middleware.RequestLog(logger))

	v1Group := s.Group("/v1")
	v1Group.Use(middleware.SetupRequired(authService))
	{
		noAuthRouter := v1Group.Group("/")
		{
			noAuthRouter.POST("/login", authHandler.Login)
			noAuthRouter.POST("/setup", authHandler.Setup)
			noAuthRouter.GET("/setup/need", authHandler.NeedSetup)
			noAuthRouter.GET("/health", dockerHandler.Health)
			noAuthRouter.GET("/robots.txt", func(c *gin.Context) {
				c.String(200, "User-agent: *\nDisallow: /")
			})
			noAuthRouter.POST("/auto-login", authHandler.AutoLogin)
		}

		strictAuthRouter := v1Group.Group("/")
		strictAuthRouter.Use(middleware.StrictAuth(j, logger, authService))
		{
			strictAuthRouter.GET("/me", authHandler.Me)
			strictAuthRouter.PUT("/me/password", authHandler.ChangePassword)
			strictAuthRouter.GET("/me/disableauth", authHandler.GetDisableAuth)
			strictAuthRouter.POST("/me/disableauth", authHandler.ToggleDisableAuth)

			strictAuthRouter.GET("/stacks/networks", stackHandler.Networks)
			strictAuthRouter.GET("/stacks", stackHandler.List)
			strictAuthRouter.POST("/stacks", stackHandler.Create)
			strictAuthRouter.GET("/stacks/:name", stackHandler.Get)
			strictAuthRouter.PUT("/stacks/:name", stackHandler.Update)
			strictAuthRouter.DELETE("/stacks/:name", stackHandler.Delete)
			strictAuthRouter.POST("/stacks/:name/:op", stackHandler.Op)
			strictAuthRouter.POST("/stacks/:name/services/:service/:op", stackHandler.ServiceOp)
			strictAuthRouter.GET("/stacks/:name/stats", stackHandler.Stats)

			strictAuthRouter.GET("/settings/globalenv", settingsHandler.GetGlobalEnv)
			strictAuthRouter.PUT("/settings/globalenv", settingsHandler.SetGlobalEnv)
			strictAuthRouter.GET("/settings/primaryhostname", settingsHandler.GetPrimaryHostname)
			strictAuthRouter.PUT("/settings/primaryhostname", settingsHandler.SetPrimaryHostname)

			strictAuthRouter.POST("/composerize", composerizeHandler.Convert)

			strictAuthRouter.GET("/terminal/:name/:type", terminalHandler.WebSocket)
		}
	}
	return s, nil
}
