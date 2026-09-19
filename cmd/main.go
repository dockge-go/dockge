// dockge 命令入口：默认启动 HTTP 服务；`reset-password` 子命令用于重置用户密码。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dockge/internal/handler"
	"dockge/internal/repository"
	"dockge/internal/server"
	"dockge/internal/service"
	"dockge/pkg/app"
	"dockge/pkg/config"
	"dockge/pkg/jwt"
	"dockge/pkg/log"
	httpx "dockge/pkg/server/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// defaultConf 是配置文件默认路径；镜像构建用 -X main.defaultConf=... 覆盖为镜像内路径，
// 使容器里无需任何参数即可执行子命令（如 docker compose run --rm dockge reset-password）。
var defaultConf = "config/local.yml"

// mustConfig 读取配置，失败即终止（启动期无可用配置时没有可继续的余地）。
func mustConfig(path string) *viper.Viper {
	conf, err := config.New(path)
	if err != nil {
		panic(err)
	}
	return conf
}

func main() {
	// 启动期 panic（配置缺失、数据卷只读、端口被占用等）转为一行可操作提示：
	// 容器用户看到的应是原因与修复方向，而不是 Go 栈回溯（do.MustInvoke 会 panic）。
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "启动失败：%v\n提示：请检查配置文件、数据卷挂载（/app/data 需可写）与端口占用。\n", r)
			os.Exit(1)
		}
	}()

	args := os.Args[1:]
	// 子命令前置：flag 包遇到首个非 flag 参数即停止解析，故这里自行解析其后的参数。
	if len(args) > 0 && args[0] == "reset-password" {
		sub := flag.NewFlagSet("reset-password", flag.ExitOnError)
		subConf := sub.String("conf", defaultConf, "config path")
		_ = sub.Parse(args[1:])
		runResetPassword(mustConfig(*subConf))
		return
	}

	envConf := flag.String("conf", defaultConf, "config path, eg: -conf ./config/local.yml")
	flag.Parse()
	// 子命令后置：-conf x reset-password 同样接受。
	if flag.Arg(0) == "reset-password" {
		runResetPassword(mustConfig(*envConf))
		return
	}
	runServer(mustConfig(*envConf))
}

// runServer 是 HTTP 服务的组合根：按顺序装配配置、日志、JWT、
// 各业务层与服务器，随后启动并阻塞直至收到退出信号。
func runServer(conf *viper.Viper) {
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

	// 启动前确保 stacks 目录存在（全新部署无需额外初始化步骤）
	repo := do.MustInvoke[*repository.Repository](injector)
	if err := repo.EnsureStacksDir(); err != nil {
		panic(err)
	}

	application := do.MustInvoke[*app.App](injector)

	logger := do.MustInvoke[*log.Logger](injector)
	// 运行时自检：容器运行时不可用（最常见是漏挂 docker socket）时栈操作会全失败，
	// 启动即告警，免得用户面对「面板正常、功能全废」无从排查。
	status := repo.Preflight(context.Background())
	if !status.Ready {
		logger.Warn().
			Str("cli", status.CLI).
			Str("error", status.Error).
			Msg("容器运行时不可用：栈操作将失败，请确认已挂载 docker socket（-v /var/run/docker.sock:/var/run/docker.sock）")
	}
	// 栈目录未挂在宿主目录上时只存在于容器可写层，重建容器即丢失：这是最隐蔽的数据丢失途径。
	if !status.Stacks.Mounted {
		logger.Warn().
			Str("stacks", status.Stacks.Path).
			Msg("栈目录未挂载宿主目录：栈只存在于容器内，重建容器会丢失，请挂载宿主目录（-v /opt/stacks:/opt/stacks）")
	}
	srv := do.MustInvoke[*httpx.Server](injector)
	// 启动日志带上栈目录：用户据此确认宿主挂载是否生效
	start := logger.Info().
		Str("host", fmt.Sprintf("http://%s:%d", conf.GetString("http.host"), conf.GetInt("http.port"))).
		Str("stacks", repo.StacksDir())
	if srv.TLSCert() != "" {
		start.Msg("dockge server start (HTTPS)")
	} else {
		start.Msg("dockge server start")
	}
	if err := application.Run(context.Background()); err != nil {
		panic(err)
	}
}
