package web

import "embed"

//go:embed dist
var assetsFS embed.FS

// Assets 返回嵌入的前端静态资源（vite build 产物 dist/）。
func Assets() embed.FS {
	return assetsFS
}
