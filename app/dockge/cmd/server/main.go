// dockge HTTP 服务入口：装配依赖并启动 Gin 服务器（Dockge 复刻版）。
package main

import (
	"context"
	"flag"
	"fmt"

	"dockge/app/dockge/internal/authoidc"
	"dockge/app/dockge/internal/handler"
	"dockge/app/dockge/internal/repository"
	"dockge/app/dockge/internal/server"
	"dockge/app/dockge/internal/service"
	"dockge/pkg/app"
	"dockge/pkg/config"
	"dockge/pkg/jwt"
	"dockge/pkg/log"
	httpx "dockge/pkg/server/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// main 是 dockge HTTP 服务的组合根：按顺序装配配置、日志、JWT、
// 各业务层与服务器，随后启动并阻塞直至收到退出信号。
func main() {
	var envConf = flag.String("conf", "config/dockge/local.yml", "config path, eg: -conf ./config/dockge/local.yml")
	flag.Parse()
	conf, err := config.New(*envConf)
	if err != nil {
		panic(err)
	}

	injector := do.New(
		func(i do.Injector) { do.ProvideValue(i, conf) },
		log.Package,
		jwt.Package,
		func(i do.Injector) {
			do.Provide(i, func(i do.Injector) (*gin.Engine, error) {
				if conf.GetString("env") == "prod" {
					gin.SetMode(gin.ReleaseMode)
				}
				return gin.New(), nil
			})
		},
		repository.Package,
		service.Package,
		handler.Package,
		server.Package,
		func(i do.Injector) {
			// 外部可达地址（供 OIDC redirect URI 等使用）：
			// 反向代理/TLS 终止在外层时，http.host:port 是内网地址，
			// MUST 通过 http.base_url 配置公网地址（如 https://dockge.example.com）。
			baseURL := conf.GetString("http.base_url")
			if baseURL == "" {
				baseURL = fmt.Sprintf("http://%s:%d",
					conf.GetString("http.host"),
					conf.GetInt("http.port"),
				)
			}
			do.Provide(i, func(i do.Injector) (*authoidc.Manager, error) {
				return authoidc.NewManager(context.Background(), conf, baseURL)
			})
		},
		func(i do.Injector) {
			do.Provide(i, func(i do.Injector) (*app.App, error) {
				return app.New(
					app.WithServer(do.MustInvoke[*httpx.Server](i)),
					app.WithLogger(do.MustInvoke[*log.Logger](i)),
					app.WithName("dockge-server"),
				), nil
			})
		},
	)
	defer func() {
		_ = injector.Shutdown()
	}()

	// 启动前确保 stacks 目录存在（未跑迁移也能直接起服务）
	if err := do.MustInvoke[*repository.Repository](injector).EnsureStacksDir(); err != nil {
		panic(err)
	}

	application := do.MustInvoke[*app.App](injector)

	logger := do.MustInvoke[*log.Logger](injector)
	srv := do.MustInvoke[*httpx.Server](injector)
	if srv.TLSCert() != "" {
		logger.Info().Str("host", fmt.Sprintf("https://%s:%d", conf.GetString("http.host"), conf.GetInt("http.port"))).Msg("dockge server start (HTTPS)")
	} else {
		logger.Info().Str("host", fmt.Sprintf("http://%s:%d", conf.GetString("http.host"), conf.GetInt("http.port"))).Msg("dockge server start")
	}
	if err := application.Run(context.Background()); err != nil {
		panic(err)
	}
}
