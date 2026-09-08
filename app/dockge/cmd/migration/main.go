// dockge 数据库迁移命令：重建 dockge_users 表并种子默认账号（破坏性）。
package main

import (
	"context"
	"flag"
	"fmt"

	"dockge/app/dockge/internal/repository"
	"dockge/app/dockge/internal/server"
	"dockge/pkg/app"
	"dockge/pkg/config"
	"dockge/pkg/log"

	"github.com/samber/do/v2"
)

// main 是迁移命令的组合根：装配依赖后运行 MigrateServer，
// 完成重建与种子灌入即退出。
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
		repository.Package,
		server.Package,
		func(i do.Injector) {
			do.Provide(i, func(i do.Injector) (*app.App, error) {
				return app.New(
					app.WithServer(do.MustInvoke[*server.MigrateServer](i)),
					app.WithLogger(do.MustInvoke[*log.Logger](i)),
					app.WithName("dockge-migrate"),
				), nil
			})
		},
	)
	defer func() {
		_ = injector.Shutdown()
	}()

	// app.Run 会阻塞等待信号；迁移完成后由 MigrateServer 的 Done 通道通知 main
	// 正常退出（os.Exit 只允许出现在 main，符合项目 Go 规范）。
	go func() {
		if err := do.MustInvoke[*app.App](injector).Run(context.Background()); err != nil {
			panic(err)
		}
	}()
	<-do.MustInvoke[*server.MigrateServer](injector).Done()
	fmt.Println("migration finished")
}
