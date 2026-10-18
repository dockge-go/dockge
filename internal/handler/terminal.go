package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"dockge/internal/repository"
	"dockge/internal/service"
	"dockge/pkg/log"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samber/do/v2"
	"regexp"
)

// upgrader 把 HTTP 连接升级为 WebSocket；站点自带 JWT 认证，放行全部来源。
// compose 服务名规则（compose spec：字母数字开头，允许 _.-），用于 exec 注入防线
var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// TerminalHandler 承载终端 WebSocket 与日志流端点。
// 每个连接独占一个 PTY 子进程，连接关闭即回收，无跨连接共享状态。
type TerminalHandler struct {
	Logger     *log.Logger            `do:""`
	Repository *repository.Repository `do:""`
}

// NewTerminalHandler 构造终端处理器，由注入容器调用。
func NewTerminalHandler(i do.Injector) (*TerminalHandler, error) {
	return do.MustInvokeStruct[*TerminalHandler](i), nil
}

// wsControlMessage 客户端发送的 JSON 控制消息（如 resize）。
type wsControlMessage struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// WebSocket 终端端点：
//   - /v1/terminal/<stack>/compose-logs 栈组合日志（docker compose logs -f）
//   - /v1/terminal/<service>/exec?stack=&shell= 容器内交互 shell（compose exec 服务名）
//
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
		h.Logger.Error().Err(err).Msg("websocket upgrade")
		return
	}

	var cmd *exec.Cmd
	readOnly := false
	switch typ {
	case "exec":
		// 上游对齐：docker compose exec <服务名> <shell>（而非 docker exec 容器 ID），
		// 栈名与 shell 经 query 下发（?stack=&shell=sh|bash）
		stack := ctx.Query("stack")
		shell := ctx.DefaultQuery("shell", "bash")
		if stack == "" || !service.StackNamePattern.MatchString(stack) {
			h.writeJSON(ctx, http.StatusBadRequest, gin.H{"error": "非法栈名: " + stack})
			return
		}
		if shell != "bash" && shell != "sh" {
			h.writeJSON(ctx, http.StatusBadRequest, gin.H{"error": "不支持的 shell: " + shell})
			return
		}
		if !serviceNamePattern.MatchString(name) {
			h.writeJSON(ctx, http.StatusBadRequest, gin.H{"error": "非法服务名: " + name})
			return
		}
		dir, dirErr := h.Repository.StackExecDir(ctx.Request.Context(), stack)
		if dirErr != nil {
			h.writeJSON(ctx, http.StatusBadGateway, gin.H{"error": "找不到栈 " + stack + " 的 compose 文件，无法进入终端"})
			_ = ws.Close()
			return
		}
		// busybox sh 在 exec 目标缺失时会直接退出而非走 || 回退，
		// 因此 bash 时先探测存在再 exec（nginx:alpine 等镜像只有 sh）；显式 sh 直连
		args := []string{"-p", stack, "exec", name}
		if shell == "bash" {
			args = append(args, "/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")
		} else {
			args = append(args, "sh")
		}
		cmd, err = h.Repository.ComposeCommand(ctx.Request.Context(), args...)
		if err != nil {
			h.writeJSON(ctx, http.StatusBadGateway, gin.H{"error": err.Error()})
			_ = ws.Close()
			return
		}
		cmd.Dir = dir
	default:
		tail := ctx.DefaultQuery("tail", "100")
		if _, err := strconv.Atoi(tail); err != nil {
			tail = "100"
		}
		dir, dirErr := h.Repository.StackExecDir(ctx.Request.Context(), name)
		if dirErr != nil {
			_ = ws.WriteMessage(websocket.TextMessage, []byte("\x1b[31m找不到栈 "+name+" 的 compose 文件，无法展示日志\x1b[0m\r\n"))
			_ = ws.Close()
			return
		}
		cmd, err = h.Repository.ComposeCommand(ctx.Request.Context(),
			"-p", name, "logs", "-f", "--tail", tail, "--no-color")
		if err != nil {
			_ = ws.WriteMessage(websocket.TextMessage, []byte("\x1b[31m"+err.Error()+"\x1b[0m\r\n"))
			_ = ws.Close()
			return
		}
		cmd.Dir = dir
		readOnly = true
	}
	h.serveSession(ws, cmd, readOnly, typ == "exec")
}

// serveSession 把子进程输出接到 WebSocket 并双向转发：
//   - usePTY=true（容器 exec）：经伪终端（creack/pty，覆盖 Linux/macOS/BSD），
//     提供真正的交互式 TTY（提示符、行编辑、Ctrl+C、窗口尺寸）
//   - usePTY=false（合并日志）：普通管道即可，无需 TTY，平台与运行时无关
func (h *TerminalHandler) serveSession(ws *websocket.Conn, cmd *exec.Cmd, readOnly, usePTY bool) {
	var (
		reader io.ReadCloser
		writer io.WriteCloser
		resize func(rows, cols uint16)
	)

	if usePTY {
		master, err := pty.Start(cmd) // 内部完成 setsid/setctty 与 stdio 绑定
		if err != nil {
			h.Logger.Error().Err(err).Msg("pty start")
			_ = ws.WriteMessage(websocket.TextMessage, []byte("\x1b[31m无法打开终端: "+err.Error()+"\x1b[0m\r\n"))
			_ = ws.Close()
			return
		}
		reader, writer = master, master
		resize = func(rows, cols uint16) {
			_ = pty.Setsize(master, &pty.Winsize{Rows: rows, Cols: cols})
		}
		resize(24, 80) // 默认窗口；随后以客户端 resize 消息为准
	} else {
		pr, pw, err := os.Pipe()
		if err != nil {
			_ = ws.WriteMessage(websocket.TextMessage, []byte("\x1b[31m"+err.Error()+"\x1b[0m\r\n"))
			_ = ws.Close()
			return
		}
		cmd.Stdout, cmd.Stderr = pw, pw
		if err := cmd.Start(); err != nil {
			_ = pr.Close()
			_ = pw.Close()
			_ = ws.WriteMessage(websocket.TextMessage, []byte("\x1b[31m"+err.Error()+"\x1b[0m\r\n"))
			_ = ws.Close()
			return
		}
		// 父进程不再持有写端：子进程退出后读端 EOF，读循环自然收尾
		_ = pw.Close()
		reader = pr
		resize = func(uint16, uint16) {}
	}

	defer func() {
		_ = reader.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
		_ = ws.Close()
	}()

	// 进程 → WebSocket：字节级转发（终端输出并非按行，逐块转发避免末行滞留）
	go func() {
		buf := make([]byte, 8192)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				if writeErr := ws.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				// 子进程退出：关闭连接即可，结束提示由前端统一输出（避免重复且能跟随界面语言）
				_ = ws.Close()
				return
			}
		}
	}()

	// WebSocket → 进程：交互输入转发；JSON 控制消息处理 resize
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
			var rows, cols uint16 = 24, 80
			if v, ok := ctrl.Data["rows"].(float64); ok && v > 0 {
				rows = uint16(v)
			}
			if v, ok := ctrl.Data["cols"].(float64); ok && v > 0 {
				cols = uint16(v)
			}
			resize(rows, cols)
			continue
		}
		if !readOnly && writer != nil {
			_, _ = writer.Write(msg)
		}
	}
}

// writeJSON 发送 JSON 响应（升级失败等非 WS 场景）。
func (h *TerminalHandler) writeJSON(ctx *gin.Context, status int, data any) {
	ctx.Header("Content-Type", "application/json")
	ctx.JSON(status, data)
}
