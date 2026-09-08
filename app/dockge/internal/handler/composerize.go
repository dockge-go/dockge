package handler

import (
	"net/http"
	"regexp"
	"strings"

	v1 "dockge/app/dockge/api/v1"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// ComposerizeHandler 承载 composerize（docker run → compose）接口。
type ComposerizeHandler struct {
	*Handler
}

var portPattern = regexp.MustCompile(`-p\s+(\S+)`)
var imagePattern = regexp.MustCompile(`(\S+)\s+(?:-|\s|$)`)
var volumePattern = regexp.MustCompile(`-v\s+([^:\s]+):([^:\s]+)`)
var envPattern = regexp.MustCompile(`-e\s+(\S+)=(\S+)`)

// NewComposerizeHandler 构造 composerize 处理器，由注入容器调用。

func NewComposerizeHandler(i do.Injector) (*ComposerizeHandler, error) {
	return &ComposerizeHandler{Handler: do.MustInvoke[*Handler](i)}, nil
}

// Convert 处理 docker run 命令到 compose.yaml 的转换请求。

func (h *ComposerizeHandler) Convert(ctx *gin.Context) {
	var req v1.ComposerizeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		v1.HandleError(ctx, http.StatusBadRequest, v1.ErrBadRequest, nil)
		return
	}
	template := convertDockerRunToCompose(req.DockerRunCommand)
	v1.HandleSuccess(ctx, v1.ComposerizeResponse{ComposeTemplate: template})
}

func convertDockerRunToCompose(cmd string) string {
	lines := []string{"services:", "  app:"}
	lines = append(lines, "    image: "+extractImage(cmd))
	lines = append(lines, "    restart: unless-stopped")

	for _, m := range portPattern.FindAllStringSubmatch(cmd, -1) {
		lines = append(lines, "    ports:")
		lines = append(lines, "      - \""+m[1]+"\"")
	}
	for _, m := range volumePattern.FindAllStringSubmatch(cmd, -1) {
		lines = append(lines, "    volumes:")
		lines = append(lines, "      - \""+m[1]+":"+m[2]+"\"")
	}
	for _, m := range envPattern.FindAllStringSubmatch(cmd, -1) {
		lines = append(lines, "    environment:")
		lines = append(lines, "      - "+m[1]+"="+m[2])
	}
	return strings.Join(lines, "\n")
}

// valueFlags 需要消费一个参数值的选项（-d 之类的布尔选项不在其中）。
var valueFlags = map[string]bool{
	"-p": true, "-v": true, "-e": true, "-l": true, "-h": true,
	"-w": true, "-u": true, "--name": true, "--network": true,
	"--entrypoint": true, "--hostname": true, "--user": true,
	"--workdir": true, "--label": true, "--env": true, "--volume": true,
	"--publish": true, "--restart": true, "--memory": true, "--cpus": true,
}

func extractImage(cmd string) string {
	parts := strings.Fields(cmd)
	// 定位 "run" 子命令，其后第一个非选项 token 即镜像名
	start := 0
	for i, p := range parts {
		if p == "run" {
			start = i + 1
			break
		}
	}
	for i := start; i < len(parts); i++ {
		p := parts[i]
		if !strings.HasPrefix(p, "-") {
			return p
		}
		if !strings.Contains(p, "=") && valueFlags[p] {
			i++ // 跳过该选项的值
		}
	}
	return "unknown"
}
