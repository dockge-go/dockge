// Package version 汇聚 dockge 的构建版本号。
package version

// Version 是 dockge 服务当前版本；默认 dev，构建时由
// -ldflags "-X dockge/app/dockge/internal/version.Version=vX.Y.Z" 注入。
var Version = "dev"
