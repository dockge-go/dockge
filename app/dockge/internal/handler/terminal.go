package handler

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"

	"dockge/app/dockge/internal/repository"
	"dockge/pkg/pty"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samber/do/v2"
)

// containerIDPattern 校验 exec 终端参数中的容器 ID/名称。
var containerIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// upgrader 把 HTTP 连接升级为 WebSocket；站点自带 JWT 认证，放行全部来源。
var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// TerminalHandler 承载终端 WebSocket 与日志流端点。
// 每个连接独占一个 PTY 子进程，连接关闭即回收，无跨连接共享状态。
type TerminalHandler struct {
	*Handler
	repo *repository.Repository
}

// NewTerminalHandler 构造终端处理器，由注入容器调用。
func NewTerminalHandler(i do.Injector) (*TerminalHandler, error) {
	return &TerminalHandler{
		Handler: do.MustInvoke[*Handler](i),
		repo:    do.MustInvoke[*repository.Repository](i),
	}, nil
}

// wsControlMessage 客户端发送的 JSON 控制消息（如 resize）。
type wsControlMessage struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// WebSocket 终端端点：
//   - /v1/terminal/<stack>/compose-logs 栈组合日志（docker compose logs -f）
//   - /v1/terminal/<container>/exec 容器内交互 shell（docker exec -it）
//
// 出于安全考虑不提供宿主 shell：终端只允许进入容器。
// 浏览器 WebSocket API 无法自定义请求头，认证依赖 StrictAuth 对 ?token= 的支持。
func (h *TerminalHandler) WebSocket(ctx *gin.Context) {
	name := ctx.Param("name")
	typ := ctx.Param("type")
	if typ != "compose-logs" && typ != "exec" {
		h.writeJSON(ctx, http.StatusBadRequest, gin.H{"error": "未知终端类型: " + typ})
		return
	}
	ws, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		h.logger.Error().Err(err).Msg("websocket upgrade")
		return
	}

	var cmd *exec.Cmd
	switch typ {
	case "exec":
		if !containerIDPattern.MatchString(name) {
			h.writeJSON(ctx, http.StatusBadRequest, gin.H{"error": "非法容器 ID: " + name})
			return
		}
		// busybox sh 在 exec 目标缺失时会直接退出而非走 || 回退，
		// 因此先探测 bash 存在再 exec（nginx:alpine 等镜像只有 sh）
		cmd = exec.CommandContext(ctx.Request.Context(), "docker", "exec", "-it", name,
			"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")
	default:
		tail := ctx.DefaultQuery("tail", "200")
		if _, err := strconv.Atoi(tail); err != nil {
			tail = "200"
		}
		dir, dirErr := h.repo.StackExecDir(ctx.Request.Context(), name)
		if dirErr != nil {
			_ = ws.WriteMessage(websocket.TextMessage, []byte("\x1b[31m找不到栈 "+name+" 的 compose 文件，无法展示日志\x1b[0m\r\n"))
			_ = ws.Close()
			return
		}
		cmd = exec.CommandContext(ctx.Request.Context(), "docker",
			"compose", "-p", name, "logs", "-f", "--tail", tail, "--no-color")
		cmd.Dir = dir
	}

	ptmx, err := pty.Open(cmd)
	if err != nil {
		h.logger.Error().Err(err).Msg("pty open")
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\x1b[31m无法打开终端: "+err.Error()+"\x1b[0m\r\n"))
		_ = ws.Close()
		return
	}
	defer func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		_ = ws.Close()
	}()

	// 默认窗口大小；后续以客户端 resize 消息为准
	_ = pty.SetWinsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

	// PTY → WebSocket：字节级转发（终端输出并非按行，逐块转发避免末行滞留）
	go func() {
		buf := make([]byte, 8192)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				if writeErr := ws.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				// 子进程退出：提示并关闭连接，让前端进入断开状态
				_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[33m会话已结束\x1b[0m\r\n"))
				_ = ws.Close()
				return
			}
		}
	}()

	// WebSocket → PTY：交互输入转发给宿主 shell；JSON 控制消息处理 resize
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return
		}
		if len(msg) == 0 {
			continue
		}
		var ctrl wsControlMessage
		if json.Unmarshal(msg, &ctrl) == nil && ctrl.Type == "resize" {
			rows, cols := uint16(24), uint16(80)
			if v, ok := ctrl.Data["rows"].(float64); ok && v > 0 {
				rows = uint16(v)
			}
			if v, ok := ctrl.Data["cols"].(float64); ok && v > 0 {
				cols = uint16(v)
			}
			_ = pty.SetWinsize(ptmx, &pty.Winsize{Rows: rows, Cols: cols})
			continue
		}
		// compose 日志为只读流，忽略输入；宿主 shell 与容器 exec 转发原始字节
		if typ != "compose-logs" {
			_, _ = ptmx.Write(msg)
		}
	}
}

// writeJSON 发送 JSON 响应（升级失败等非 WS 场景）。
func (h *TerminalHandler) writeJSON(ctx *gin.Context, status int, data any) {
	ctx.Header("Content-Type", "application/json")
	ctx.JSON(status, data)
}
