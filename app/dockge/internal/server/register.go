package server

import (
	"dockge/app/dockge/internal/push"

	"github.com/samber/do/v2"
)

var Package = do.Package(
	do.Lazy(push.NewStatusHub),
	do.Lazy(NewHTTPServer),
	do.Lazy(NewContainerStatusServer),
	do.Lazy(NewMigrateServer),
)
