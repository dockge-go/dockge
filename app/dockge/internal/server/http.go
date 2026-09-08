package server

import (
	nethttp "net/http"
	"strings"

	"dockge/app/dockge/internal/handler"
	"dockge/app/dockge/internal/middleware"
	"dockge/app/dockge/web"
	"dockge/pkg/jwt"
	"dockge/pkg/log"
	httpx "dockge/pkg/server/http"

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
	{
		noAuthRouter := v1Group.Group("/")
		{
			noAuthRouter.POST("/login", authHandler.Login)
			noAuthRouter.POST("/setup", authHandler.Setup)
			noAuthRouter.GET("/setup/need", authHandler.NeedSetup)
			noAuthRouter.POST("/2fa", authHandler.Check2FA)
			noAuthRouter.GET("/health", dockerHandler.Health)
			noAuthRouter.GET("/robots.txt", func(c *gin.Context) {
				c.String(200, "User-agent: *\nDisallow: /")
			})
		}

		strictAuthRouter := v1Group.Group("/").Use(middleware.StrictAuth(j, logger))
		{
			strictAuthRouter.GET("/me", authHandler.Me)
			strictAuthRouter.PUT("/me/password", authHandler.ChangePassword)
			strictAuthRouter.POST("/me/2fa/enable", authHandler.Enable2FA)
			strictAuthRouter.DELETE("/me/2fa", authHandler.Disable2FA)
			strictAuthRouter.GET("/me/disableauth", authHandler.GetDisableAuth)
			strictAuthRouter.POST("/me/disableauth", authHandler.ToggleDisableAuth)

			noAuthRouter.POST("/auto-login", authHandler.AutoLogin)

			strictAuthRouter.GET("/stacks", stackHandler.List)
			strictAuthRouter.POST("/stacks", stackHandler.Create)
			strictAuthRouter.GET("/stacks/:name", stackHandler.Get)
			strictAuthRouter.PUT("/stacks/:name", stackHandler.Update)
			strictAuthRouter.DELETE("/stacks/:name", stackHandler.Delete)
			strictAuthRouter.POST("/stacks/:name/:op", stackHandler.Op)

			strictAuthRouter.GET("/docker/version", dockerHandler.Version)
			strictAuthRouter.GET("/docker/containers", dockerHandler.Containers)
			strictAuthRouter.GET("/docker/containers/stream", dockerHandler.ContainerStatusStream)
			strictAuthRouter.GET("/docker/containers/:id/inspect", dockerHandler.ContainerInspect)
			strictAuthRouter.GET("/docker/info", dockerHandler.Info)
			strictAuthRouter.GET("/docker/networks", dockerHandler.Networks)
			strictAuthRouter.GET("/docker/networks/:name", dockerHandler.NetworkInspect)
			strictAuthRouter.DELETE("/docker/networks/:name", dockerHandler.RemoveNetwork)
			strictAuthRouter.GET("/docker/images", dockerHandler.DockerImages)
			strictAuthRouter.DELETE("/docker/images/:id", dockerHandler.RemoveImage)
			strictAuthRouter.POST("/docker/images/pull", dockerHandler.PullImage)
			strictAuthRouter.POST("/docker/images/prune", dockerHandler.PruneImages)
			strictAuthRouter.POST("/docker/networks/create", dockerHandler.NetworkCreate)
			strictAuthRouter.POST("/docker/containers/prune", dockerHandler.PruneContainers)
			strictAuthRouter.POST("/docker/networks/prune", dockerHandler.PruneNetworks)
			strictAuthRouter.POST("/docker/volumes/prune", dockerHandler.PruneVolumes)
			strictAuthRouter.GET("/docker/volumes", dockerHandler.DockerVolumes)
			strictAuthRouter.DELETE("/docker/volumes/:name", dockerHandler.RemoveVolume)
			strictAuthRouter.GET("/docker/stats/stream", dockerHandler.StatsStream)
			strictAuthRouter.GET("/docker/df", dockerHandler.DockerDf)
			strictAuthRouter.POST("/docker/containers/:id/stop", dockerHandler.StopContainer)
			strictAuthRouter.POST("/docker/containers/:id/start", dockerHandler.StartContainer)
			strictAuthRouter.POST("/docker/containers/:id/restart", dockerHandler.RestartContainer)
			strictAuthRouter.GET("/docker/containers/:id/logs", dockerHandler.ContainerLogs)
			strictAuthRouter.DELETE("/docker/containers/:id", dockerHandler.RemoveContainer)
			strictAuthRouter.GET("/version/check", dockerHandler.VersionCheck)

			strictAuthRouter.GET("/settings/globalenv", settingsHandler.GetGlobalEnv)
			strictAuthRouter.PUT("/settings/globalenv", settingsHandler.SetGlobalEnv)

			strictAuthRouter.POST("/composerize", composerizeHandler.Convert)

			strictAuthRouter.GET("/terminal/:name/:type", terminalHandler.WebSocket)
		}
	}
	return s, nil
}
