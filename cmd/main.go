// dockge 命令入口：启动 HTTP 服务。单用户：首启（bbolt 尚无用户）经 Web 引导创建管理员；
// 忘记密码只能删除 bbolt 数据文件重新部署（库中仅账号与设置，栈文件无损）。
// 无配置文件：一切配置来自内置默认值与 DOCKGE_* 环境变量（见 pkg/config）。
package main

import (
	"context"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dockge/internal/handler"
	"dockge/internal/repository"
	"dockge/internal/server"
	"dockge/internal/service"
	"dockge/pkg/config"
	"dockge/pkg/jwt"
	"dockge/pkg/log"

	"github.com/samber/do/v2"
	"github.com/spf13/viper"
)

// basePackage 注册基础设施（配置/日志/JWT），其余各层自带 Package。
func basePackage(conf *viper.Viper) func(do.Injector) {
	return do.Package(
		do.Lazy(func(i do.Injector) (*viper.Viper, error) { return conf, nil }),
		do.Lazy(func(i do.Injector) (*log.Logger, error) { return log.NewLogger(conf), nil }),
		do.Lazy(func(i do.Injector) (*jwt.JWT, error) { return jwt.NewJWT(conf), nil }),
	)
}

// mustLoadConfig 加载配置（默认值 + DOCKGE_* 环境变量），失败即终止。
func mustLoadConfig() *viper.Viper {
	conf, err := config.Load()
	if err != nil {
		panic(err)
	}
	return conf
}

func main() {
	// 启动期 panic（数据卷只读、端口被占用等）转为一行可操作提示：
	// 容器用户看到的应是原因与修复方向，而不是 Go 栈回溯（do.MustInvoke 会 panic）。
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "启动失败：%v\n提示：请检查环境变量、数据卷挂载（/app/data 需可写）与端口占用。\n", r)
			os.Exit(1)
		}
	}()

	conf := mustLoadConfig()
	container := do.New(
		basePackage(conf),
		repository.Package,
		service.Package,
		handler.Package,
		server.Package,
	)

	// 启动前确保 stacks 目录存在（全新部署无需额外初始化步骤）。
	// 顺带触发 repository 构造：JWT 密钥（显式环境变量 > 库中持久值 > 首启随机生成）
	// 在此写回 viper，后续 jwt 惰性解析时拿到的即最终值。
	repo := do.MustInvoke[*repository.Repository](container)
	if err := repo.EnsureStacksDir(); err != nil {
		panic(err)
	}

	httpSrv := do.MustInvoke[*server.HTTPServer](container)
	logger := do.MustInvoke[*log.Logger](container)

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
	// 启动日志带上栈目录：用户据此确认宿主挂载是否生效
	start := logger.Info().
		Str("http", fmt.Sprintf("http://%s:%d", conf.GetString("http.host"), conf.GetInt("http.port"))).
		Str("stacks", repo.StacksDir())
	if httpSrv.TLSEnabled() {
		start.Msg("dockge server start (HTTPS)")
	} else {
		start.Msg("dockge server start")
	}

	go func() {
		if err := httpSrv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "启动失败：%v\n提示：请检查端口占用与 TLS 证书配置。\n", err)
			os.Exit(1)
		}
	}()
	logger.Info().Str("http", httpSrv.Addr).Msg("server started")

	// 等待退出信号（SIGINT/SIGTERM）。不能给等待用的 ctx 设超时：
	// samber/do 的 ShutdownOnSignalsWithContext 在 ctx 到期时同样触发关闭，
	// 预设 10s 超时会让服务无信号定时自杀。
	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-sigCtx.Done()

	// 关闭限时 10s：容器停止时逐项停服务（HTTP 优雅退出等）
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	report := container.ShutdownWithContext(shutdownCtx)
	if !report.Succeed {
		stdlog.Printf("shutdown report: %v", report)
	}
}
