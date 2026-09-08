package server

import (
	"dockge/app/dockge/internal/push"
	"dockge/app/dockge/internal/service"

	"github.com/samber/do/v2"
)

var Package = do.Package(
	do.Lazy(push.NewStatusHub),
	do.Lazy(NewHTTPServer),
	do.Lazy(NewContainerStatusServer),
	do.Lazy(NewMigrateServer),
)

// StartBackgroundTasks 启动后台定时任务（Settings缓存清理器等）。
func StartBackgroundTasks(i do.Injector) {
	service.StartSettingsCleaner(i)
}
